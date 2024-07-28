/*
 * UPF Event Exposure Service
 *
 * UPF Event Exposure Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.1.0-alpha.5
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package nupf

// Data within UPF Create Event Subscription Response
type CreatedEventSubscription struct {
	Subscription *UpfEventSubscription `json:"subscription"`

	SubscriptionId string `json:"subscriptionId"`

	ReportList []NotificationItem `json:"reportList,omitempty"`

	SupportedFeatures string `json:"supportedFeatures,omitempty"`
}
