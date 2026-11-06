package identity

import (
	"context"
	"testing"
)

// TestListTOTPEnrollmentStats_CountsReflectRealEnrollment covers
// platform-security-hardening task 2.10's backend data: creating a
// staff member and enrolling them must move them from "total" into
// "enrolled" for their studio.
func TestListTOTPEnrollmentStats_CountsReflectRealEnrollment(t *testing.T) {
	repo, _, studioID := testRepo(t)
	ctx := context.Background()

	role, err := repo.CreateRole(ctx, studioID, "test-role-stats-"+studioID.String(), "", nil)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })

	statsBefore := findStudioStats(t, repo, studioID)

	userID, err := repo.CreateStudioUser(ctx, studioID, "enrollment-stats-test", "Test", "User",
		"enrollment-stats-test-"+studioID.String()+"@example.com", role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
	})

	afterCreate := findStudioStats(t, repo, studioID)
	wantTotal := 1
	if statsBefore != nil {
		wantTotal = statsBefore.TotalStaff + 1
	}
	if afterCreate == nil || afterCreate.TotalStaff != wantTotal {
		t.Fatalf("TotalStaff after creating one staff member = %+v, want total %d", afterCreate, wantTotal)
	}
	enrolledBefore := 0
	if statsBefore != nil {
		enrolledBefore = statsBefore.EnrolledStaff
	}
	if afterCreate.EnrolledStaff != enrolledBefore {
		t.Fatalf("EnrolledStaff should be unchanged before enrollment: got %d, want %d", afterCreate.EnrolledStaff, enrolledBefore)
	}

	cipher := testCipher(t)
	h := &Handler{repo: repo, cipher: cipher}
	enrollTOTPForTest(t, h, userID)

	afterEnroll := findStudioStats(t, repo, studioID)
	if afterEnroll == nil || afterEnroll.EnrolledStaff != enrolledBefore+1 {
		t.Fatalf("EnrolledStaff after enrollment = %+v, want %d", afterEnroll, enrolledBefore+1)
	}
	if afterEnroll.TotalStaff != wantTotal {
		t.Errorf("TotalStaff should not change on enrollment: got %d, want %d", afterEnroll.TotalStaff, wantTotal)
	}
}

func findStudioStats(t *testing.T, repo *Repo, studioID interface{ String() string }) *StudioTOTPEnrollmentStats {
	t.Helper()
	stats, err := repo.ListTOTPEnrollmentStats(context.Background())
	if err != nil {
		t.Fatalf("ListTOTPEnrollmentStats: %v", err)
	}
	for i := range stats {
		if stats[i].StudioID.String() == studioID.String() {
			return &stats[i]
		}
	}
	return nil
}
