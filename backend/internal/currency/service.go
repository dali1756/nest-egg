package currency

import "context"

type Currency struct {
	Code      string
	Name      string
	MinorUnit int
}

type Store interface {
	List(ctx context.Context) ([]Currency, error)
	Exists(ctx context.Context, code string) (bool, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// List 依代碼排序回傳所有幣別。
func (s *Service) List(ctx context.Context) ([]Currency, error) {
	return s.store.List(ctx)
}

// Exists 回報 code 是不是 currencies 表裡的幣別。
func (s *Service) Exists(ctx context.Context, code string) (bool, error) {
	return s.store.Exists(ctx, code)
}
