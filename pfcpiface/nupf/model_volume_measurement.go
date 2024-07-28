/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

// Volume Measurement information
type VolumeMeasurement struct {
	TotalVolume string `json:"totalVolume"`

	UlVolume string `json:"ulVolume"`

	DlVolume string `json:"dlVolume"`

	TotalNbOfPackets uint64 `json:"totalNbOfPackets"`

	UlNbOfPackets uint64 `json:"ulNbOfPackets"`

	DlNbOfPackets uint64 `json:"dlNbOfPackets"`
}
