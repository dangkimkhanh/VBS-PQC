package common

import "errors"

var (
	//auth
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidToken = errors.New("invalid_token")

	ErrNoFieldsToUpdate               = errors.New("no_fields_to_update")
	ErrUserNotExisted                 = errors.New("user_not_exists")
	ErrInvalidUserID                  = errors.New("invalid_user_id")
	ErrStudentIDExists                = errors.New("student_id_exists")
	ErrEmailExists                    = errors.New("email_exists")
	ErrUniversityNameExists           = errors.New("university_name_exists")
	ErrUniversityEmailDomainExists    = errors.New("university_email_domain_exists")
	ErrUniversityCodeExists           = errors.New("university_code_exists")
	ErrUniversityNotFound             = errors.New("university not found")
	ErrAccountUniversityNotFound      = errors.New("university account not found")
	ErrAccountUniversityAlreadyExists = errors.New("university_admin_account_already_exists")
	ErrUniversityApprovalEmailFailed  = errors.New("activation_email_failed")
	ErrAccountNotFound                = errors.New("account_not_found")
	ErrInvalidOldPassword             = errors.New("invalid_old_password")
	ErrPersonalAccountAlreadyExist    = errors.New("personal_account_already_exists")
	ErrCheckingPersonalAccount        = errors.New("error_checking_personal_account")
	ErrRateLimited                    = errors.New("rate_limited")

	//Faculty
	ErrFacultyNotFound   = errors.New("faculty_not_found")
	ErrFacultyCodeExists = errors.New("faculty_code_existed")

	ErrTemplateNotFound                 = errors.New("template_not_found")
	ErrEDiplomaAlreadyExists            = errors.New("ediploma_already_exists")
	ErrEDiplomaAccessDenied             = errors.New("ediploma_access_denied")
	ErrMissingRequiredFieldsForEDiploma = errors.New("missing_required_fields_for_ediploma")

	//General
	ErrNotFound = errors.New("not_found")

	ErrInvalidFaculty = errors.New("invalid faculty_id")
)

var (
	ErrNoDiplomas      = errors.New("không tìm thấy bằng số")
	ErrNoValidDiplomas = errors.New("no valid eDiplomas to push")
	ErrAlreadyOnChain  = errors.New("id đã tồn tại")
	ErrMissingHash     = errors.New("ediploma chưa có hash")
	ErrBatchNotFound   = errors.New("batch không tồn tại")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
