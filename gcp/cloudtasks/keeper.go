package cloudtasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cloudtasksapi "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	eywa "github.com/wmulabs/eywa"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudTasksConfig holds the configuration for the Cloud Tasks scheduler.
type CloudTasksConfig struct {
	Project             string // GCP project ID
	Location            string // Cloud Tasks region (e.g. "us-central1")
	Queue               string // Cloud Tasks queue name
	TargetBaseURL       string // Base URL of the eywa service (e.g. "https://my-service.run.app")
	TargetPath          string // HTTP path Cloud Tasks POSTs to. Defaults to "/internal/execute-event" when empty.
	TargetAudience      string // OIDC audience for Cloud Run auth (optional; defaults to TargetBaseURL)
	ServiceAccountEmail string // Service account for OIDC token (optional; defaults to Cloud Run identity)
}

type CloudTasksKeeper struct {
	client *cloudtasksapi.Client
	config CloudTasksConfig
}

func NewCloudTasksKeeper(ctx context.Context, cfg CloudTasksConfig) (eywa.Keeper, error) {
	client, err := cloudtasksapi.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud tasks client: %w", err)
	}
	return &CloudTasksKeeper{client: client, config: cfg}, nil
}

// Schedule: pass time.Now() for immediate async dispatch; future time for scheduled execution.
func (s *CloudTasksKeeper) Schedule(ctx context.Context, eventKey string, event *eywa.Pulse, executeAt time.Time) (string, error) {
	body, err := json.Marshal(scheduledEventPayload{EventKey: eventKey, Event: event})
	if err != nil {
		return "", fmt.Errorf("marshal event: %w", err)
	}

	req := &cloudtaskspb.CreateTaskRequest{
		Parent: s.queuePath(),
		Task: &cloudtaskspb.Task{
			ScheduleTime: timestamppb.New(executeAt),
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: s.buildHTTPRequest(body),
			},
		},
	}

	task, err := s.client.CreateTask(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create cloud task: %w", err)
	}

	return task.GetName(), nil
}

// Cancel deletes a pending task. Returns nil if the task no longer exists.
func (s *CloudTasksKeeper) Cancel(ctx context.Context, taskID string) error {
	err := s.client.DeleteTask(ctx, &cloudtaskspb.DeleteTaskRequest{Name: taskID})
	if isNotFound(err) {
		return nil
	}
	return err
}

func (s *CloudTasksKeeper) buildHTTPRequest(body []byte) *cloudtaskspb.HttpRequest {
	path := s.config.TargetPath
	if path == "" {
		path = "/internal/execute-event"
	}

	req := &cloudtaskspb.HttpRequest{
		HttpMethod: cloudtaskspb.HttpMethod_POST,
		Url:        s.config.TargetBaseURL + path,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}

	if s.config.TargetAudience != "" {
		req.AuthorizationHeader = &cloudtaskspb.HttpRequest_OidcToken{
			OidcToken: &cloudtaskspb.OidcToken{
				ServiceAccountEmail: s.config.ServiceAccountEmail,
				Audience:            s.config.TargetAudience,
			},
		}
	}

	return req
}

func (s *CloudTasksKeeper) queuePath() string {
	return fmt.Sprintf("projects/%s/locations/%s/queues/%s",
		s.config.Project, s.config.Location, s.config.Queue)
}

func isNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

// scheduledEventPayload is the body sent by Cloud Tasks to the internal endpoint.
type scheduledEventPayload struct {
	EventKey string      `json:"event_key"`
	Event    *eywa.Pulse `json:"event"`
}
