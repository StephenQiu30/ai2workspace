package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/refunds"
)

type learningSaleSnapshotDTO struct {
	CourseID         string `json:"courseId"`
	PublishedVersion int    `json:"publishedVersion"`
	Title            string `json:"title"`
	SaleState        string `json:"saleState"`
	Deliverable      bool   `json:"deliverable"`
	IsFree           bool   `json:"isFree"`
}

type learningDeliveryEvidenceDTO struct {
	CourseID           string     `json:"courseId"`
	FirstDeliverableAt *time.Time `json:"firstDeliverableAt"`
	EvidenceVersion    int        `json:"evidenceVersion"`
	ContentVersion     int        `json:"contentVersion"`
}

type communityNotificationDTO struct {
	EventID     string `json:"eventId"`
	RecipientID string `json:"recipientId"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
}

// NewLearningSaleVerifier builds an HTTP sale-snapshot verifier targeting the learning service.
func NewLearningSaleVerifier(learningBaseURL, serviceToken string, httpClient *http.Client) orders.SaleSnapshotVerifier {
	baseURL := strings.TrimRight(strings.TrimSpace(learningBaseURL), "/")
	if baseURL == "" {
		return nil
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	return func(ctx context.Context, courseID string) (*orders.SaleSnapshot, error) {
		endpoint := fmt.Sprintf("%s/internal/learning/courses/%s/sale-snapshot", baseURL, url.PathEscape(courseID))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set(auth.ServiceTokenHeader, serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			var dto learningSaleSnapshotDTO
			if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
				return nil, fmt.Errorf("invalid sale-snapshot response: %w", err)
			}
			return &orders.SaleSnapshot{
				CourseID:         dto.CourseID,
				Title:            dto.Title,
				SaleState:        dto.SaleState,
				Deliverable:      dto.Deliverable,
				IsFree:           dto.IsFree,
				PublishedVersion: dto.PublishedVersion,
			}, nil
		case http.StatusNotFound:
			return nil, orders.ErrCourseNotFound
		default:
			return nil, fmt.Errorf("learning sale-snapshot returned status %d", resp.StatusCode)
		}
	}
}

// NewLearningDeliveryEvidenceProvider builds an HTTP delivery-evidence provider targeting the learning service.
func NewLearningDeliveryEvidenceProvider(learningBaseURL, serviceToken string, httpClient *http.Client) refunds.DeliveryEvidenceProvider {
	baseURL := strings.TrimRight(strings.TrimSpace(learningBaseURL), "/")
	if baseURL == "" {
		return nil
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	return func(ctx context.Context, courseID string, notBefore time.Time) (*time.Time, error) {
		endpoint := fmt.Sprintf("%s/internal/learning/courses/%s/delivery-evidence?notBefore=%s",
			baseURL, url.PathEscape(courseID), url.QueryEscape(notBefore.UTC().Format(time.RFC3339)))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set(auth.ServiceTokenHeader, serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("learning delivery-evidence returned status %d", resp.StatusCode)
		}

		var dto learningDeliveryEvidenceDTO
		if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
			return nil, fmt.Errorf("invalid delivery-evidence response: %w", err)
		}
		return dto.FirstDeliverableAt, nil
	}
}

// NewCommunityNotifier builds a fault-tolerant HTTP notification client targeting the community service.
func NewCommunityNotifier(communityBaseURL, serviceToken string, httpClient *http.Client) func(ctx context.Context, eventID, recipientID, notifType, title, summary string) error {
	baseURL := strings.TrimRight(strings.TrimSpace(communityBaseURL), "/")
	if baseURL == "" {
		return nil
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := baseURL + "/internal/community/notifications"

	return func(ctx context.Context, eventID, recipientID, notifType, title, summary string) error {
		payload, err := json.Marshal(communityNotificationDTO{
			EventID:     eventID,
			RecipientID: recipientID,
			Type:        notifType,
			Title:       title,
			Summary:     summary,
		})
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(auth.ServiceTokenHeader, serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("community notification returned status %d", resp.StatusCode)
		}
		return nil
	}
}
