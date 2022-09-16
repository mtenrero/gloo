// Code generated by solo-kit. DO NOT EDIT.

package v1

import (
	"sync"
	"time"

	github_com_solo_io_solo_kit_pkg_api_v1_resources_common_kubernetes "github.com/solo-io/solo-kit/pkg/api/v1/resources/common/kubernetes"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"

	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	"github.com/solo-io/solo-kit/pkg/api/v1/resources"
	"github.com/solo-io/solo-kit/pkg/errors"
	skstats "github.com/solo-io/solo-kit/pkg/stats"

	"github.com/solo-io/go-utils/contextutils"
	"github.com/solo-io/go-utils/errutils"
)

var (
	// Deprecated. See mDiscoveryResourcesIn
	mDiscoverySnapshotIn = stats.Int64("discovery.gloo.solo.io/emitter/snap_in", "Deprecated. Use discovery.gloo.solo.io/emitter/resources_in. The number of snapshots in", "1")

	// metrics for emitter
	mDiscoveryResourcesIn    = stats.Int64("discovery.gloo.solo.io/emitter/resources_in", "The number of resource lists received on open watch channels", "1")
	mDiscoverySnapshotOut    = stats.Int64("discovery.gloo.solo.io/emitter/snap_out", "The number of snapshots out", "1")
	mDiscoverySnapshotMissed = stats.Int64("discovery.gloo.solo.io/emitter/snap_missed", "The number of snapshots missed", "1")

	// views for emitter
	// deprecated: see discoveryResourcesInView
	discoverysnapshotInView = &view.View{
		Name:        "discovery.gloo.solo.io/emitter/snap_in",
		Measure:     mDiscoverySnapshotIn,
		Description: "Deprecated. Use discovery.gloo.solo.io/emitter/resources_in. The number of snapshots updates coming in.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}

	discoveryResourcesInView = &view.View{
		Name:        "discovery.gloo.solo.io/emitter/resources_in",
		Measure:     mDiscoveryResourcesIn,
		Description: "The number of resource lists received on open watch channels",
		Aggregation: view.Count(),
		TagKeys: []tag.Key{
			skstats.NamespaceKey,
			skstats.ResourceKey,
		},
	}
	discoverysnapshotOutView = &view.View{
		Name:        "discovery.gloo.solo.io/emitter/snap_out",
		Measure:     mDiscoverySnapshotOut,
		Description: "The number of snapshots updates going out",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
	discoverysnapshotMissedView = &view.View{
		Name:        "discovery.gloo.solo.io/emitter/snap_missed",
		Measure:     mDiscoverySnapshotMissed,
		Description: "The number of snapshots updates going missed. this can happen in heavy load. missed snapshot will be re-tried after a second.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
)

func init() {
	view.Register(
		discoverysnapshotInView,
		discoverysnapshotOutView,
		discoverysnapshotMissedView,
		discoveryResourcesInView,
	)
}

type DiscoverySnapshotEmitter interface {
	Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *DiscoverySnapshot, <-chan error, error)
}

type DiscoveryEmitter interface {
	DiscoverySnapshotEmitter
	Register() error
	Upstream() UpstreamClient
	KubeNamespace() github_com_solo_io_solo_kit_pkg_api_v1_resources_common_kubernetes.KubeNamespaceClient
	Secret() SecretClient
}

func NewDiscoveryEmitter(upstreamClient UpstreamClient, kubeNamespaceClient github_com_solo_io_solo_kit_pkg_api_v1_resources_common_kubernetes.KubeNamespaceClient, secretClient SecretClient, resourceNamespaceLister resources.ResourceNamespaceLister) DiscoveryEmitter {
	return NewDiscoveryEmitterWithEmit(upstreamClient, kubeNamespaceClient, secretClient, resourceNamespaceLister, make(chan struct{}))
}

func NewDiscoveryEmitterWithEmit(upstreamClient UpstreamClient, kubeNamespaceClient github_com_solo_io_solo_kit_pkg_api_v1_resources_common_kubernetes.KubeNamespaceClient, secretClient SecretClient, resourceNamespaceLister resources.ResourceNamespaceLister, emit <-chan struct{}) DiscoveryEmitter {
	return &discoveryEmitter{
		upstream:                upstreamClient,
		kubeNamespace:           kubeNamespaceClient,
		secret:                  secretClient,
		resourceNamespaceLister: resourceNamespaceLister,
		forceEmit:               emit,
	}
}

type discoveryEmitter struct {
	forceEmit     <-chan struct{}
	upstream      UpstreamClient
	kubeNamespace github_com_solo_io_solo_kit_pkg_api_v1_resources_common_kubernetes.KubeNamespaceClient
	secret        SecretClient
	// resourceNamespaceLister is used to watch for new namespaces when they are created.
	// It is used when Expression Selector is in the Watch Opts set in Snapshot().
	resourceNamespaceLister resources.ResourceNamespaceLister
	// namespacesWatching is the set of namespaces that we are watching. This is helpful
	// when Expression Selector is set on the Watch Opts in Snapshot().
	namespacesWatching sync.Map
	// updateNamespaces is used to perform locks and unlocks when watches on namespaces are being updated/created
	updateNamespaces sync.Mutex
}

func (c *discoveryEmitter) Register() error {
	if err := c.upstream.Register(); err != nil {
		return err
	}
	if err := c.kubeNamespace.Register(); err != nil {
		return err
	}
	if err := c.secret.Register(); err != nil {
		return err
	}
	return nil
}

func (c *discoveryEmitter) Upstream() UpstreamClient {
	return c.upstream
}

func (c *discoveryEmitter) KubeNamespace() github_com_solo_io_solo_kit_pkg_api_v1_resources_common_kubernetes.KubeNamespaceClient {
	return c.kubeNamespace
}

func (c *discoveryEmitter) Secret() SecretClient {
	return c.secret
}

// Snapshots will return a channel that can be used to receive snapshots of the
// state of the resources it is watching
// when watching resources, you can set the watchNamespaces, and you can set the
// ExpressionSelector of the WatchOpts.  Setting watchNamespaces will watch for all resources
// that are in the specified namespaces. In addition if ExpressionSelector of the WatchOpts is
// set, then all namespaces that meet the label criteria of the ExpressionSelector will
// also be watched.
func (c *discoveryEmitter) Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *DiscoverySnapshot, <-chan error, error) {

	if len(watchNamespaces) == 0 {
		watchNamespaces = []string{""}
	}

	for _, ns := range watchNamespaces {
		if ns == "" && len(watchNamespaces) > 1 {
			return nil, nil, errors.Errorf("the \"\" namespace is used to watch all namespaces. Snapshots can either be tracked for " +
				"specific namespaces or \"\" AllNamespaces, but not both.")
		}
	}

	errs := make(chan error)
	hasWatchedNamespaces := len(watchNamespaces) > 1 || (len(watchNamespaces) == 1 && watchNamespaces[0] != "")
	watchingLabeledNamespaces := !(opts.ExpressionSelector == "")
	var done sync.WaitGroup
	ctx := opts.Ctx

	// setting up the options for both listing and watching resources in namespaces
	watchedNamespacesListOptions := clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector}
	watchedNamespacesWatchOptions := clients.WatchOpts{Ctx: opts.Ctx, Selector: opts.Selector}
	/* Create channel for Upstream */
	type upstreamListWithNamespace struct {
		list      UpstreamList
		namespace string
	}
	upstreamChan := make(chan upstreamListWithNamespace)
	var initialUpstreamList UpstreamList
	/* Create channel for KubeNamespace */
	/* Create channel for Secret */
	type secretListWithNamespace struct {
		list      SecretList
		namespace string
	}
	secretChan := make(chan secretListWithNamespace)
	var initialSecretList SecretList

	currentSnapshot := DiscoverySnapshot{}
	upstreamsByNamespace := sync.Map{}
	secretsByNamespace := sync.Map{}
	if hasWatchedNamespaces || !watchingLabeledNamespaces {
		// then watch all resources on watch Namespaces

		// watched namespaces
		for _, namespace := range watchNamespaces {
			/* Setup namespaced watch for Upstream */
			{
				upstreams, err := c.upstream.List(namespace, watchedNamespacesListOptions)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "initial Upstream list")
				}
				initialUpstreamList = append(initialUpstreamList, upstreams...)
				upstreamsByNamespace.Store(namespace, upstreams)
			}
			upstreamNamespacesChan, upstreamErrs, err := c.upstream.Watch(namespace, watchedNamespacesWatchOptions)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "starting Upstream watch")
			}

			done.Add(1)
			go func(namespace string) {
				defer done.Done()
				errutils.AggregateErrs(ctx, errs, upstreamErrs, namespace+"-upstreams")
			}(namespace)
			/* Setup namespaced watch for Secret */
			{
				secrets, err := c.secret.List(namespace, watchedNamespacesListOptions)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "initial Secret list")
				}
				initialSecretList = append(initialSecretList, secrets...)
				secretsByNamespace.Store(namespace, secrets)
			}
			secretNamespacesChan, secretErrs, err := c.secret.Watch(namespace, watchedNamespacesWatchOptions)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "starting Secret watch")
			}

			done.Add(1)
			go func(namespace string) {
				defer done.Done()
				errutils.AggregateErrs(ctx, errs, secretErrs, namespace+"-secrets")
			}(namespace)
			/* Watch for changes and update snapshot */
			go func(namespace string) {
				defer func() {
					c.namespacesWatching.Delete(namespace)
				}()
				c.namespacesWatching.Store(namespace, true)
				for {
					select {
					case <-ctx.Done():
						return
					case upstreamList, ok := <-upstreamNamespacesChan:
						if !ok {
							return
						}
						select {
						case <-ctx.Done():
							return
						case upstreamChan <- upstreamListWithNamespace{list: upstreamList, namespace: namespace}:
						}
					case secretList, ok := <-secretNamespacesChan:
						if !ok {
							return
						}
						select {
						case <-ctx.Done():
							return
						case secretChan <- secretListWithNamespace{list: secretList, namespace: namespace}:
						}
					}
				}
			}(namespace)
		}
	}
	// watch all other namespaces that are labeled and fit the Expression Selector
	if opts.ExpressionSelector != "" {
		// watch resources of non-watched namespaces that fit the expression selectors
		namespaceListOptions := resources.ResourceNamespaceListOptions{
			Ctx:                opts.Ctx,
			ExpressionSelector: opts.ExpressionSelector,
		}
		namespaceWatchOptions := resources.ResourceNamespaceWatchOptions{
			Ctx:                opts.Ctx,
			ExpressionSelector: opts.ExpressionSelector,
		}

		filterNamespaces := resources.ResourceNamespaceList{}
		for _, ns := range watchNamespaces {
			// we do not want to filter out "" which equals all namespaces
			// the reason is because we will never create a watch on ""(all namespaces) because
			// doing so means we watch all resources regardless of namespace. Our intent is to
			// watch only certain namespaces.
			if ns != "" {
				filterNamespaces = append(filterNamespaces, resources.ResourceNamespace{Name: ns})
			}
		}
		namespacesResources, err := c.resourceNamespaceLister.GetResourceNamespaceList(namespaceListOptions, filterNamespaces)
		if err != nil {
			return nil, nil, err
		}
		newlyRegisteredNamespaces := make([]string, len(namespacesResources))
		// non watched namespaces that are labeled
		for i, resourceNamespace := range namespacesResources {
			namespace := resourceNamespace.Name
			newlyRegisteredNamespaces[i] = namespace
			err = c.upstream.RegisterNamespace(namespace)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "there was an error registering the namespace to the upstream")
			}
			/* Setup namespaced watch for Upstream */
			{
				upstreams, err := c.upstream.List(namespace, clients.ListOpts{Ctx: opts.Ctx})
				if err != nil {
					return nil, nil, errors.Wrapf(err, "initial Upstream list with new namespace")
				}
				initialUpstreamList = append(initialUpstreamList, upstreams...)
				upstreamsByNamespace.Store(namespace, upstreams)
			}
			upstreamNamespacesChan, upstreamErrs, err := c.upstream.Watch(namespace, clients.WatchOpts{Ctx: opts.Ctx})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "starting Upstream watch")
			}

			done.Add(1)
			go func(namespace string) {
				defer done.Done()
				errutils.AggregateErrs(ctx, errs, upstreamErrs, namespace+"-upstreams")
			}(namespace)
			err = c.secret.RegisterNamespace(namespace)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "there was an error registering the namespace to the secret")
			}
			/* Setup namespaced watch for Secret */
			{
				secrets, err := c.secret.List(namespace, clients.ListOpts{Ctx: opts.Ctx})
				if err != nil {
					return nil, nil, errors.Wrapf(err, "initial Secret list with new namespace")
				}
				initialSecretList = append(initialSecretList, secrets...)
				secretsByNamespace.Store(namespace, secrets)
			}
			secretNamespacesChan, secretErrs, err := c.secret.Watch(namespace, clients.WatchOpts{Ctx: opts.Ctx})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "starting Secret watch")
			}

			done.Add(1)
			go func(namespace string) {
				defer done.Done()
				errutils.AggregateErrs(ctx, errs, secretErrs, namespace+"-secrets")
			}(namespace)
			/* Watch for changes and update snapshot */
			go func(namespace string) {
				for {
					select {
					case <-ctx.Done():
						return
					case upstreamList, ok := <-upstreamNamespacesChan:
						if !ok {
							return
						}
						select {
						case <-ctx.Done():
							return
						case upstreamChan <- upstreamListWithNamespace{list: upstreamList, namespace: namespace}:
						}
					case secretList, ok := <-secretNamespacesChan:
						if !ok {
							return
						}
						select {
						case <-ctx.Done():
							return
						case secretChan <- secretListWithNamespace{list: secretList, namespace: namespace}:
						}
					}
				}
			}(namespace)
		}
		if len(newlyRegisteredNamespaces) > 0 {
			contextutils.LoggerFrom(ctx).Infof("registered the new namespace %v", newlyRegisteredNamespaces)
		}

		// create watch on all namespaces, so that we can add all resources from new namespaces
		// we will be watching namespaces that meet the Expression Selector filter

		namespaceWatch, errsReceiver, err := c.resourceNamespaceLister.GetResourceNamespaceWatch(namespaceWatchOptions, filterNamespaces)
		if err != nil {
			return nil, nil, err
		}
		if errsReceiver != nil {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case err = <-errsReceiver:
						errs <- errors.Wrapf(err, "received error from watch on resource namespaces")
					}
				}
			}()
		}

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case resourceNamespaces, ok := <-namespaceWatch:
					if !ok {
						return
					}
					// get the list of new namespaces, if there is a new namespace
					// get the list of resources from that namespace, and add
					// a watch for new resources created/deleted on that namespace
					c.updateNamespaces.Lock()

					// get the new namespaces, and get a map of the namespaces
					mapOfResourceNamespaces := make(map[string]bool, len(resourceNamespaces))
					newNamespaces := []string{}
					for _, ns := range resourceNamespaces {
						if _, hit := c.namespacesWatching.Load(ns.Name); !hit {
							newNamespaces = append(newNamespaces, ns.Name)
						}
						mapOfResourceNamespaces[ns.Name] = true
					}

					for _, ns := range watchNamespaces {
						mapOfResourceNamespaces[ns] = true
					}

					missingNamespaces := []string{}
					// use the map of namespace resources to find missing/deleted namespaces
					c.namespacesWatching.Range(func(key interface{}, value interface{}) bool {
						name := key.(string)
						if _, hit := mapOfResourceNamespaces[name]; !hit {
							missingNamespaces = append(missingNamespaces, name)
						}
						return true
					})

					for _, ns := range missingNamespaces {
						upstreamChan <- upstreamListWithNamespace{list: UpstreamList{}, namespace: ns}
						secretChan <- secretListWithNamespace{list: SecretList{}, namespace: ns}
					}

					for _, namespace := range newNamespaces {
						var err error
						err = c.upstream.RegisterNamespace(namespace)
						if err != nil {
							errs <- errors.Wrapf(err, "there was an error registering the namespace to the upstream")
							continue
						}
						/* Setup namespaced watch for Upstream for new namespace */
						{
							upstreams, err := c.upstream.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
							if err != nil {
								errs <- errors.Wrapf(err, "initial new namespace Upstream list in namespace watch")
								continue
							}
							upstreamsByNamespace.Store(namespace, upstreams)
						}
						upstreamNamespacesChan, upstreamErrs, err := c.upstream.Watch(namespace, clients.WatchOpts{Ctx: opts.Ctx, Selector: opts.Selector})
						if err != nil {
							errs <- errors.Wrapf(err, "starting new namespace Upstream watch")
							continue
						}

						done.Add(1)
						go func(namespace string) {
							defer done.Done()
							errutils.AggregateErrs(ctx, errs, upstreamErrs, namespace+"-new-namespace-upstreams")
						}(namespace)
						err = c.secret.RegisterNamespace(namespace)
						if err != nil {
							errs <- errors.Wrapf(err, "there was an error registering the namespace to the secret")
							continue
						}
						/* Setup namespaced watch for Secret for new namespace */
						{
							secrets, err := c.secret.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
							if err != nil {
								errs <- errors.Wrapf(err, "initial new namespace Secret list in namespace watch")
								continue
							}
							secretsByNamespace.Store(namespace, secrets)
						}
						secretNamespacesChan, secretErrs, err := c.secret.Watch(namespace, clients.WatchOpts{Ctx: opts.Ctx, Selector: opts.Selector})
						if err != nil {
							errs <- errors.Wrapf(err, "starting new namespace Secret watch")
							continue
						}

						done.Add(1)
						go func(namespace string) {
							defer done.Done()
							errutils.AggregateErrs(ctx, errs, secretErrs, namespace+"-new-namespace-secrets")
						}(namespace)
						/* Watch for changes and update snapshot */
						go func(namespace string) {
							defer func() {
								c.namespacesWatching.Delete(namespace)
							}()
							c.namespacesWatching.Store(namespace, true)
							for {
								select {
								case <-ctx.Done():
									return
								case upstreamList, ok := <-upstreamNamespacesChan:
									if !ok {
										return
									}
									select {
									case <-ctx.Done():
										return
									case upstreamChan <- upstreamListWithNamespace{list: upstreamList, namespace: namespace}:
									}
								case secretList, ok := <-secretNamespacesChan:
									if !ok {
										return
									}
									select {
									case <-ctx.Done():
										return
									case secretChan <- secretListWithNamespace{list: secretList, namespace: namespace}:
									}
								}
							}
						}(namespace)
					}
					if len(newNamespaces) > 0 {
						contextutils.LoggerFrom(ctx).Infof("registered the new namespace %v", newNamespaces)
						c.updateNamespaces.Unlock()
					}
				}
			}
		}()
	}
	/* Initialize snapshot for Upstreams */
	currentSnapshot.Upstreams = initialUpstreamList.Sort()
	/* Setup cluster-wide watch for KubeNamespace */
	var err error
	currentSnapshot.Kubenamespaces, err = c.kubeNamespace.List(clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "initial KubeNamespace list")
	}
	// for Cluster scoped resources, we do not use Expression Selectors
	kubeNamespaceChan, kubeNamespaceErrs, err := c.kubeNamespace.Watch(clients.WatchOpts{
		Ctx:      opts.Ctx,
		Selector: opts.Selector,
	})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "starting KubeNamespace watch")
	}
	done.Add(1)
	go func() {
		defer done.Done()
		errutils.AggregateErrs(ctx, errs, kubeNamespaceErrs, "kubenamespaces")
	}()
	/* Initialize snapshot for Secrets */
	currentSnapshot.Secrets = initialSecretList.Sort()

	snapshots := make(chan *DiscoverySnapshot)
	go func() {
		// sent initial snapshot to kick off the watch
		initialSnapshot := currentSnapshot.Clone()
		snapshots <- &initialSnapshot

		timer := time.NewTicker(time.Second * 1)
		previousHash, err := currentSnapshot.Hash(nil)
		if err != nil {
			contextutils.LoggerFrom(ctx).Panicw("error while hashing, this should never happen", zap.Error(err))
		}
		sync := func() {
			currentHash, err := currentSnapshot.Hash(nil)
			// this should never happen, so panic if it does
			if err != nil {
				contextutils.LoggerFrom(ctx).Panicw("error while hashing, this should never happen", zap.Error(err))
			}
			if previousHash == currentHash {
				return
			}

			sentSnapshot := currentSnapshot.Clone()
			select {
			case snapshots <- &sentSnapshot:
				stats.Record(ctx, mDiscoverySnapshotOut.M(1))
				previousHash = currentHash
			default:
				stats.Record(ctx, mDiscoverySnapshotMissed.M(1))
			}
		}

		defer func() {
			close(snapshots)
			// we must wait for done before closing the error chan,
			// to avoid sending on close channel.
			done.Wait()
			close(errs)
		}()
		for {
			record := func() { stats.Record(ctx, mDiscoverySnapshotIn.M(1)) }

			select {
			case <-timer.C:
				sync()
			case <-ctx.Done():
				return
			case <-c.forceEmit:
				sentSnapshot := currentSnapshot.Clone()
				snapshots <- &sentSnapshot
			case upstreamNamespacedList, ok := <-upstreamChan:
				if !ok {
					return
				}
				record()

				namespace := upstreamNamespacedList.namespace

				skstats.IncrementResourceCount(
					ctx,
					namespace,
					"upstream",
					mDiscoveryResourcesIn,
				)

				// merge lists by namespace
				upstreamsByNamespace.Store(namespace, upstreamNamespacedList.list)
				var upstreamList UpstreamList
				upstreamsByNamespace.Range(func(key interface{}, value interface{}) bool {
					mocks := value.(UpstreamList)
					upstreamList = append(upstreamList, mocks...)
					return true
				})
				currentSnapshot.Upstreams = upstreamList.Sort()
			case kubeNamespaceList, ok := <-kubeNamespaceChan:
				if !ok {
					return
				}
				record()

				skstats.IncrementResourceCount(
					ctx,
					"<all>",
					"kube_namespace",
					mDiscoveryResourcesIn,
				)

				currentSnapshot.Kubenamespaces = kubeNamespaceList
			case secretNamespacedList, ok := <-secretChan:
				if !ok {
					return
				}
				record()

				namespace := secretNamespacedList.namespace

				skstats.IncrementResourceCount(
					ctx,
					namespace,
					"secret",
					mDiscoveryResourcesIn,
				)

				// merge lists by namespace
				secretsByNamespace.Store(namespace, secretNamespacedList.list)
				var secretList SecretList
				secretsByNamespace.Range(func(key interface{}, value interface{}) bool {
					mocks := value.(SecretList)
					secretList = append(secretList, mocks...)
					return true
				})
				currentSnapshot.Secrets = secretList.Sort()
			}
		}
	}()
	return snapshots, errs, nil
}
