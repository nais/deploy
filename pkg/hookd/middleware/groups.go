package middleware

type GroupProvider int

const (
	GroupProviderInvalid GroupProvider = iota // Default value is invalid to force explicit choice when used in structs
	GroupProviderGoogle
	GroupProviderAzure
)
