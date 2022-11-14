// Code generated by protoc-gen-ext. DO NOT EDIT.
// source: github.com/solo-io/gloo/projects/gloo/api/v1/options/retries/retries.proto

package retries

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	equality "github.com/solo-io/protoc-gen-ext/pkg/equality"
)

// ensure the imports are used
var (
	_ = errors.New("")
	_ = fmt.Print
	_ = binary.LittleEndian
	_ = bytes.Compare
	_ = strings.Compare
	_ = equality.Equalizer(nil)
	_ = proto.Message(nil)
)

// Equal function
func (m *RetryPolicyInterval) Equal(that interface{}) bool {
	if that == nil {
		return m == nil
	}

	target, ok := that.(*RetryPolicyInterval)
	if !ok {
		that2, ok := that.(RetryPolicyInterval)
		if ok {
			target = &that2
		} else {
			return false
		}
	}
	if target == nil {
		return m == nil
	} else if m == nil {
		return false
	}

	if h, ok := interface{}(m.GetBaseInterval()).(equality.Equalizer); ok {
		if !h.Equal(target.GetBaseInterval()) {
			return false
		}
	} else {
		if !proto.Equal(m.GetBaseInterval(), target.GetBaseInterval()) {
			return false
		}
	}

	if h, ok := interface{}(m.GetMaxInterval()).(equality.Equalizer); ok {
		if !h.Equal(target.GetMaxInterval()) {
			return false
		}
	} else {
		if !proto.Equal(m.GetMaxInterval(), target.GetMaxInterval()) {
			return false
		}
	}

	return true
}

// Equal function
func (m *RetryPolicy) Equal(that interface{}) bool {
	if that == nil {
		return m == nil
	}

	target, ok := that.(*RetryPolicy)
	if !ok {
		that2, ok := that.(RetryPolicy)
		if ok {
			target = &that2
		} else {
			return false
		}
	}
	if target == nil {
		return m == nil
	} else if m == nil {
		return false
	}

	if strings.Compare(m.GetRetryOn(), target.GetRetryOn()) != 0 {
		return false
	}

	if m.GetNumRetries() != target.GetNumRetries() {
		return false
	}

	if h, ok := interface{}(m.GetPerTryTimeout()).(equality.Equalizer); ok {
		if !h.Equal(target.GetPerTryTimeout()) {
			return false
		}
	} else {
		if !proto.Equal(m.GetPerTryTimeout(), target.GetPerTryTimeout()) {
			return false
		}
	}

	if h, ok := interface{}(m.GetRetryPolicyInterval()).(equality.Equalizer); ok {
		if !h.Equal(target.GetRetryPolicyInterval()) {
			return false
		}
	} else {
		if !proto.Equal(m.GetRetryPolicyInterval(), target.GetRetryPolicyInterval()) {
			return false
		}
	}

	return true
}
