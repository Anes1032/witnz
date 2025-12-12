package verify

import "fmt"

type TamperingError struct {
	TableName string
	Operation string
	Message   string
}

func (e *TamperingError) Error() string {
	return fmt.Sprintf("TAMPERING DETECTED: %s operation on %s table %s",
		e.Operation, e.Message, e.TableName)
}

func (e *TamperingError) IsTampering() bool {
	return true
}

func (e *TamperingError) GetTableName() string {
	return e.TableName
}

func (e *TamperingError) GetOperation() string {
	return e.Operation
}

func NewTamperingError(tableName, operation, message string) *TamperingError {
	return &TamperingError{
		TableName: tableName,
		Operation: operation,
		Message:   message,
	}
}

func IsTamperingError(err error) bool {
	_, ok := err.(*TamperingError)
	return ok
}

func AsTamperingError(err error) *TamperingError {
	if te, ok := err.(*TamperingError); ok {
		return te
	}
	return nil
}
