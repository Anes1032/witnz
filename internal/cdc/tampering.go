package cdc

type TamperingDetector interface {
	error
	IsTampering() bool
	GetTableName() string
	GetOperation() string
}
