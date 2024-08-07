/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

import (
	"time"
)

const ()

// UPF Event Mode
type UpfEventMode struct {
	Trigger string `json:"trigger"` // ONE_TIME PERIODIC

	MaxReports int32 `json:"maxReports,omitempty"`

	Expiry *time.Time `json:"expiry,omitempty"`

	RepPeriod uint32 `json:"repPeriod,omitempty"`

	SampRatio int32 `json:"sampRatio,omitempty"`

	FlowSampling *FlowSampling `json:"flowSampling,omitempty"`

	PartitioningCriteria []string `json:"partitioningCriteria,omitempty"` // TAC SUBPLMN GEOAREA SNSSAI DNN

	NotifFlag string `json:"notifFlag,omitempty"` // Possible values are: - ACTIVATE: The event notification is activated. - DEACTIVATE: The event notification is deactivated and shall be muted. The available    event(s) shall be stored. - RETRIEVAL: The event notification shall be sent to the NF service consumer(s), after that, is muted again.

	MutingExcInstructions *AllOfUpfEventModeMutingExcInstructions `json:"mutingExcInstructions,omitempty"`

	MutingNotSettings *AllOfUpfEventModeMutingNotSettings `json:"mutingNotSettings,omitempty"`
}
