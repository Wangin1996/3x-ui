package xray

import (
	"encoding/json"
	"fmt"
)

func ApplyHotDiff(api *XrayAPI, diff *HotDiff) error {
	for _, tag := range diff.RemovedInboundTags {
		if err := api.DelInbound(tag); err != nil && !IsMissingHandlerErr(err) {
			return fmt.Errorf("remove inbound [%s]: %w", tag, err)
		}
	}
	for _, tag := range diff.RemovedOutboundTags {
		if err := api.DelOutbound(tag); err != nil && !IsMissingHandlerErr(err) {
			return fmt.Errorf("remove outbound [%s]: %w", tag, err)
		}
	}
	for _, ob := range diff.AddedOutbounds {
		if err := addOutboundReconciling(api, ob); err != nil {
			return fmt.Errorf("add outbound: %w", err)
		}
	}
	for _, ib := range diff.AddedInbounds {
		if err := addInboundReconciling(api, ib); err != nil {
			return fmt.Errorf("add inbound: %w", err)
		}
	}
	if diff.RoutingConfig != nil {
		if err := api.ApplyRoutingConfig(diff.RoutingConfig); err != nil {
			return fmt.Errorf("apply routing config: %w", err)
		}
	}
	return nil
}

func addInboundReconciling(api *XrayAPI, inbound []byte) error {
	err := api.AddInbound(inbound)
	if err == nil || !IsExistingTagErr(err) {
		return err
	}
	var meta struct {
		Tag string `json:"tag"`
	}
	if jsonErr := json.Unmarshal(inbound, &meta); jsonErr != nil || meta.Tag == "" {
		return err
	}
	if delErr := api.DelInbound(meta.Tag); delErr != nil && !IsMissingHandlerErr(delErr) {
		return delErr
	}
	return api.AddInbound(inbound)
}

func addOutboundReconciling(api *XrayAPI, outbound []byte) error {
	err := api.AddOutbound(outbound)
	if err == nil || !IsExistingTagErr(err) {
		return err
	}
	var meta struct {
		Tag string `json:"tag"`
	}
	if jsonErr := json.Unmarshal(outbound, &meta); jsonErr != nil || meta.Tag == "" {
		return err
	}
	if delErr := api.DelOutbound(meta.Tag); delErr != nil && !IsMissingHandlerErr(delErr) {
		return delErr
	}
	return api.AddOutbound(outbound)
}
