package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

type stubOperatorRepo struct {
	operators map[string]*eywa.Operator
	err       error
	updateErr error
}

func newStubOperatorRepo() *stubOperatorRepo {
	return &stubOperatorRepo{operators: make(map[string]*eywa.Operator)}
}

func (s *stubOperatorRepo) Create(_ context.Context, op *eywa.Operator) error {
	if s.err != nil {
		return s.err
	}
	if op.ID == "" {
		op.ID = "generated-id"
	}
	s.operators[op.ID] = op
	return nil
}

func (s *stubOperatorRepo) FindByEmail(_ context.Context, email string) (*eywa.Operator, error) {
	for _, op := range s.operators {
		if op.Email == email {
			return op, nil
		}
	}
	return nil, eywa.ErrNotFound
}

func (s *stubOperatorRepo) FindByID(_ context.Context, id string) (*eywa.Operator, error) {
	op, ok := s.operators[id]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return op, s.err
}

func (s *stubOperatorRepo) List(_ context.Context, _, _ int) ([]*eywa.Operator, int64, error) {
	ops := make([]*eywa.Operator, 0, len(s.operators))
	for _, op := range s.operators {
		ops = append(ops, op)
	}
	return ops, int64(len(ops)), s.err
}

func (s *stubOperatorRepo) Update(_ context.Context, op *eywa.Operator) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if s.err != nil {
		return s.err
	}
	if _, ok := s.operators[op.ID]; !ok {
		return eywa.ErrNotFound
	}
	s.operators[op.ID] = op
	return nil
}

func (s *stubOperatorRepo) Deactivate(_ context.Context, id string) error {
	if s.err != nil {
		return s.err
	}
	op, ok := s.operators[id]
	if !ok {
		return eywa.ErrNotFound
	}
	op.IsActive = false
	return nil
}

func seedOperator(repo *stubOperatorRepo, id, email, role string) *eywa.Operator {
	hash, _ := eywa.HashPassword("secret")
	op := &eywa.Operator{
		ID:           id,
		Name:         "Test Op",
		Email:        email,
		Role:         role,
		PasswordHash: hash,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	repo.operators[id] = op
	return op
}

func adminDeps(repo *stubOperatorRepo) ManagementDeps {
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	return ManagementDeps{
		APIKeys:      map[string]string{"admin-key": eywa.RoleAdmin},
		OperatorAuth: auth,
	}
}

func TestOperatorHandler_Login_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	app := buildMgmtTestApp(ManagementDeps{OperatorAuth: auth})

	body := map[string]string{"email": "admin@test.com", "password": "secret"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == nil {
		t.Error("want token in response, got nil")
	}
}

func TestOperatorHandler_Login_WrongPassword_Returns401(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	app := buildMgmtTestApp(ManagementDeps{OperatorAuth: auth})

	body := map[string]string{"email": "admin@test.com", "password": "wrong"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_List_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/operators", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["items"] == nil {
		t.Error("want items in response")
	}
}

func TestOperatorHandler_List_ForbiddenForOperatorRole_Returns403(t *testing.T) {
	repo := newStubOperatorRepo()
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	app := buildMgmtTestApp(ManagementDeps{
		APIKeys:      map[string]string{"op-key": eywa.RoleOperator},
		OperatorAuth: auth,
	})

	req := httptest.NewRequest("GET", "/api/v1/operators", nil)
	req.Header.Set("Authorization", "Bearer op-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 403 {
		t.Errorf("want 403, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Create_Returns201(t *testing.T) {
	repo := newStubOperatorRepo()
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "Alice", "email": "alice@test.com", "password": "pass123", "role": eywa.RoleOperator}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] == nil {
		t.Error("want id in response")
	}
}

func TestOperatorHandler_Create_MissingFields_Returns400(t *testing.T) {
	repo := newStubOperatorRepo()
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "Alice"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_GetByID_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/operators/op-1", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Deactivate_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/operators/op-1", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.operators["op-1"].IsActive {
		t.Error("want operator to be deactivated")
	}
}

func TestOperatorHandler_Update_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "Updated Name"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/operators/op-1", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.operators["op-1"].Name != "Updated Name" {
		t.Errorf("want Updated Name, got %s", repo.operators["op-1"].Name)
	}
}

func TestOperatorHandler_Update_NotFound_Returns404(t *testing.T) {
	repo := newStubOperatorRepo()
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "X"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/operators/no-such-id", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_GetByID_NotFound_Returns404(t *testing.T) {
	repo := newStubOperatorRepo()
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/operators/no-such-id", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Deactivate_NotFound_Returns404(t *testing.T) {
	repo := newStubOperatorRepo()
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/operators/no-such-id", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Login_InvalidBody_Returns400(t *testing.T) {
	repo := newStubOperatorRepo()
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	app := buildMgmtTestApp(ManagementDeps{OperatorAuth: auth})

	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_List_RepoError_Returns500(t *testing.T) {
	repo := newStubOperatorRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/operators", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Create_RepoError_Returns500(t *testing.T) {
	repo := newStubOperatorRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "Alice", "email": "a@b.com", "password": "pass", "role": eywa.RoleOperator}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_GetByID_GenericError_Returns500(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "a@b.com", eywa.RoleAdmin)
	repo.err = errInternal
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/operators/op-1", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Update_FindGenericError_Returns500(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "a@b.com", eywa.RoleAdmin)
	repo.err = errInternal
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "x"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/operators/op-1", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Deactivate_GenericError_Returns500(t *testing.T) {
	repo := newStubOperatorRepo()
	repo.err = errInternal
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/operators/op-1", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Create_HashPasswordError_Returns500(t *testing.T) {
	// bcrypt fails for passwords > 72 bytes
	tooLongPassword := strings.Repeat("a", 73)
	app := buildMgmtTestApp(adminDeps(newStubOperatorRepo()))

	body := map[string]string{
		"name":     "Test",
		"email":    "test@test.com",
		"password": tooLongPassword,
		"role":     "admin",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500 for bcrypt error, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Create_InvalidBody_Returns400(t *testing.T) {
	app := buildMgmtTestApp(adminDeps(newStubOperatorRepo()))

	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader([]byte("bad")))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Login_MissingFields_Returns400(t *testing.T) {
	repo := newStubOperatorRepo()
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	app := buildMgmtTestApp(ManagementDeps{OperatorAuth: auth})

	body := map[string]string{"email": "admin@test.com"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Update_InvalidBody_Returns400(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	app := buildMgmtTestApp(adminDeps(repo))

	req := httptest.NewRequest("PUT", "/api/v1/operators/op-1", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Update_EmailAndRole_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"email": "new@test.com", "role": string(eywa.RoleOperator)}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/operators/op-1", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	if repo.operators["op-1"].Email != "new@test.com" {
		t.Errorf("want email new@test.com, got %s", repo.operators["op-1"].Email)
	}
	if repo.operators["op-1"].Role != eywa.RoleOperator {
		t.Errorf("want role operator, got %s", repo.operators["op-1"].Role)
	}
}

func TestOperatorHandler_Update_UpdateRepoError_Returns500(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	repo.updateErr = errInternal
	app := buildMgmtTestApp(adminDeps(repo))

	body := map[string]string{"name": "New Name"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/operators/op-1", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}
