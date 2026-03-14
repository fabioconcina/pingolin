package exitcode

const (
	Success      = 0 // Normal exit, no errors.
	GeneralError = 1 // Config load failure, database error, I/O error, etc.
	Unhealthy    = 2 // Connection unhealthy — at least one probe failing (status command only).
)
