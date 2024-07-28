/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

// Throughput Measurement information
type ThroughputMeasurement struct {
	UlThroughput string `json:"ulThroughput,omitempty"`

	DlThroughput string `json:"dlThroughput,omitempty"`

	UlPacketThroughput string `json:"ulPacketThroughput,omitempty"`

	DlPacketThroughput string `json:"dlPacketThroughput,omitempty"`
}