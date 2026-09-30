package models

// UniversityStats are per-university counters shown to the platform admin.
type UniversityStats struct {
	Students          int64 `json:"students"`
	ActivatedStudents int64 `json:"activated_students"`
	Diplomas          int64 `json:"diplomas"`
	Issued            int64 `json:"issued"`
	Signed            int64 `json:"signed"`
	Anchored          int64 `json:"anchored"`
	Revoked           int64 `json:"revoked"`
}

// UniversityListFilter drives the searchable, paginated university list.
// Status is one of "", "active", "pending" (admin not activated yet) or "locked".
type UniversityListFilter struct {
	Query    string `form:"q"`
	Status   string `form:"status" binding:"omitempty,oneof=active pending locked"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type UniversityListItem struct {
	UniversityResponse
	Stats UniversityStats `json:"stats"`
}

type PlatformOverview struct {
	Universities struct {
		Total        int64 `json:"total"`
		Active       int64 `json:"active"`
		Locked       int64 `json:"locked"`
		AdminPending int64 `json:"admin_pending"`
	} `json:"universities"`
	Students struct {
		Total     int64 `json:"total"`
		Activated int64 `json:"activated"`
	} `json:"students"`
	Diplomas struct {
		Total    int64 `json:"total"`
		Issued   int64 `json:"issued"`
		Signed   int64 `json:"signed"`
		Anchored int64 `json:"anchored"`
		Revoked  int64 `json:"revoked"`
	} `json:"diplomas"`
	Verifications struct {
		Attempts int64 `json:"attempts"`
		Passed   int64 `json:"passed"`
	} `json:"verifications"`
	// Algorithms counts active signing keys per ML-DSA parameter set.
	Algorithms map[string]int64 `json:"algorithms"`
	// TopUniversities lists the universities that issued the most diplomas.
	TopUniversities []UniversityListItem `json:"top_universities"`
	// PendingActivations lists universities whose admin never activated the account.
	PendingActivations []UniversityResponse `json:"pending_activations"`
}

type AdminAuditEvent struct {
	At             string `json:"at"`
	Action         string `json:"action"`
	Method         string `json:"method"`
	Success        bool   `json:"success"`
	UniversityID   string `json:"university_id"`
	UniversityName string `json:"university_name"`
	UniversityCode string `json:"university_code"`
}
