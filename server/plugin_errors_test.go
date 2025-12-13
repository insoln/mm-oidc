package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestFriendlyProvisioningErrorLastAdmin(t *testing.T) {
	appErr := &model.AppError{
		Id:            "api.user.demote_last_admin.app_error",
		Message:       "Cannot demote last System Admin.",
		DetailedError: "Cannot demote last System Admin.",
		StatusCode:    http.StatusBadRequest,
		Where:         "UpdateUserRoles",
	}

	wrapped := fmt.Errorf("update user roles: %w", appErr)

	status, title, message, handled := friendlyProvisioningError(wrapped)
	if !handled {
		t.Fatalf("expected error to be handled")
	}

	if status != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, status)
	}

	if title != "Cannot remove last System Admin" {
		t.Fatalf("unexpected title: %s", title)
	}

	if !strings.Contains(strings.ToLower(message), "system administrator") {
		t.Fatalf("expected message to mention system administrator: %s", message)
	}
}

func TestFriendlyProvisioningErrorUnhandled(t *testing.T) {
	if _, _, _, handled := friendlyProvisioningError(errors.New("boom")); handled {
		t.Fatalf("expected error to remain unhandled")
	}
}
