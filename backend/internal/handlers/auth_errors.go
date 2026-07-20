package handlers

type externalAuthError struct {
	status  int
	message string
}

func (e externalAuthError) Error() string {
	return e.message
}
