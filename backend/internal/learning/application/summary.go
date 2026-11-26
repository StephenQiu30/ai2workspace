package application

import (
	"context"
	"time"

	coursesapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/courses"
	mediaapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/media"
	progressapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/progress"
	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
	mediadomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/media"
)

// SummarizeProgress reads lesson and media state to calculate course progress.
func SummarizeProgress(ctx context.Context, progressStore progressapp.Store, mediaStore mediaapp.Store, userID string, course *coursesdomain.Course) (coursesapp.CourseProgressSummary, error) {
	summary := coursesapp.CourseProgressSummary{
		TotalLessonCount: course.LessonCount,
	}
	records, err := progressStore.ListProgressByCourse(ctx, userID, course.ID)
	if err != nil {
		return summary, err
	}

	lessonTypes := make(map[string]string, course.LessonCount)
	lessonMediaIDs := make(map[string]string, course.LessonCount)
	for _, sec := range course.Sections {
		for _, l := range sec.Lessons {
			lessonTypes[l.ID] = l.Type
			lessonMediaIDs[l.ID] = l.MediaVersionID
		}
	}

	var latestTime *time.Time
	watchedSet := make(map[string]struct{}, len(records))
	completedSet := make(map[string]struct{}, len(records))
	for _, rec := range records {
		watchedSet[rec.LessonID] = struct{}{}
		if latestTime == nil || rec.UpdatedAt.After(*latestTime) {
			t := rec.UpdatedAt
			latestTime = &t
			summary.LastLessonID = rec.LessonID
			summary.LastPositionSeconds = rec.PositionSeconds
		}

		lType := lessonTypes[rec.LessonID]
		if lType == "article" {
			completedSet[rec.LessonID] = struct{}{}
		} else if rec.PositionSeconds > 0 {
			var mv *mediadomain.MediaVersion
			if mvID := lessonMediaIDs[rec.LessonID]; mvID != "" {
				mv, _ = mediaStore.GetMediaVersion(ctx, mvID)
			}
			if mv == nil {
				mv, _ = mediaStore.GetLessonMediaVersion(ctx, rec.LessonID)
			}
			if mv != nil && mv.DurationSeconds > 0 && rec.PositionSeconds >= mv.DurationSeconds {
				completedSet[rec.LessonID] = struct{}{}
			}
		}
	}

	summary.WatchedLessonCount = len(watchedSet)
	summary.CompletedLessonCount = len(completedSet)
	summary.LastStudiedAt = latestTime
	return summary, nil
}
