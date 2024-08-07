/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

// Provides additional information in an error response.
type ProblemDetails struct {
	Type_ string `json:"type,omitempty"`

	Title string `json:"title,omitempty"`

	Status int32 `json:"status,omitempty"`
	// A human-readable explanation specific to this occurrence of the problem.
	Detail string `json:"detail,omitempty"`

	Instance string `json:"instance,omitempty"`
	// A machine-readable application error cause specific to this occurrence of the problem.  This IE should be present and provide application-related error information, if available.
	Cause string `json:"cause,omitempty"`

	InvalidParams []InvalidParam `json:"invalidParams,omitempty"`

	SupportedFeatures string `json:"supportedFeatures,omitempty"`

	AccessTokenError *AccessTokenErr `json:"accessTokenError,omitempty"`

	AccessTokenRequest *AccessTokenReq `json:"accessTokenRequest,omitempty"`

	NrfId string `json:"nrfId,omitempty"`

	SupportedApiVersions []string `json:"supportedApiVersions,omitempty"`

	NoProfileMatchInfo *NoProfileMatchInfo `json:"noProfileMatchInfo,omitempty"`
}
