package clusters

import "errors"

var (
	// ErrDynamicClientRequired is returned when no dynamic client is provided
	ErrDynamicClientRequired = errors.New("dynamic client is required")

	// ErrNamespaceRequired is returned when namespace is required but not provided
	ErrNamespaceRequired = errors.New("namespace is required in OIDC_REQUIRED mode")

	// ErrNamespaceForbidden is returned when a namespace is not allowed by the filter
	ErrNamespaceForbidden = errors.New("namespace not allowed by namespace filter")

	// ErrNoAllowedNamespaces is returned when no namespaces match the filter
	ErrNoAllowedNamespaces = errors.New("no allowed namespaces found")

	// ErrResourceNotFound is returned when a requested resource does not exist
	ErrResourceNotFound = errors.New("resource not found")

	// ErrInvalidRequest is returned when request validation fails
	ErrInvalidRequest = errors.New("invalid request")
)
