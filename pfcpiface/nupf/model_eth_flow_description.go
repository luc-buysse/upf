/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

// Identifies an Ethernet flow.
type EthFlowDescription struct {
	DestMacAddr string `json:"destMacAddr,omitempty"`

	EthType string `json:"ethType"`

	FDesc string `json:"fDesc,omitempty"`

	FDir string `json:"fDir,omitempty"` // DOWNLINK UPLINK BIDIRECTIONAL UNSPECIFIED

	SourceMacAddr string `json:"sourceMacAddr,omitempty"`

	VlanTags []string `json:"vlanTags,omitempty"`

	SrcMacAddrEnd string `json:"srcMacAddrEnd,omitempty"`

	DestMacAddrEnd string `json:"destMacAddrEnd,omitempty"`
}