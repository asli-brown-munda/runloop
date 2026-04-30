package inbox

import (
	"context"

	"runloop/internal/sources"
)

type Repository interface {
	UpsertInboxItem(context.Context, sources.InboxCandidate) (InboxItem, InboxItemVersion, bool, error)
	GetInboxItem(context.Context, int64) (InboxItem, error)
	ListInboxItems(context.Context) ([]InboxItem, error)
	ArchiveInboxItem(context.Context, int64) error
	IgnoreInboxItem(context.Context, int64) error
	LatestInboxVersion(context.Context, int64) (InboxItemVersion, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UpsertInboxItem(ctx context.Context, c sources.InboxCandidate) (InboxItem, InboxItemVersion, bool, error) {
	return s.repo.UpsertInboxItem(ctx, c)
}

func (s *Service) GetInboxItem(ctx context.Context, id int64) (InboxItem, error) {
	return s.repo.GetInboxItem(ctx, id)
}

func (s *Service) ListInboxItems(ctx context.Context) ([]InboxItem, error) {
	return s.repo.ListInboxItems(ctx)
}

func (s *Service) ArchiveInboxItem(ctx context.Context, id int64) error {
	return s.repo.ArchiveInboxItem(ctx, id)
}

func (s *Service) IgnoreInboxItem(ctx context.Context, id int64) error {
	return s.repo.IgnoreInboxItem(ctx, id)
}

func (s *Service) LatestInboxVersion(ctx context.Context, itemID int64) (InboxItemVersion, error) {
	return s.repo.LatestInboxVersion(ctx, itemID)
}
