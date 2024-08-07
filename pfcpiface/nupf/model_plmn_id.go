/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

// When PlmnId needs to be converted to string (e.g. when used in maps as key), the string  shall be composed of three digits \"mcc\" followed by \"-\" and two or three digits \"mnc\".
type PlmnId struct {
	Mcc string `json:"mcc"`

	Mnc string `json:"mnc"`
}
