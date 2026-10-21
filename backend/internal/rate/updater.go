package rate

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"nest-egg/backend/internal/platform/taipei"
)

// 央行在工作日 16:00～17:00 公布當日收盤，所以 17:30 抓，還沒有就每小時重試到 22:30
const (
	dailyHour, dailyMinute = 17, 30
	lastHour, lastMinute   = 22, 30
	retryInterval          = time.Hour
)

func nextRun(now time.Time, haveToday bool) time.Time {
	local := now.In(taipei.Location)
	y, m, d := local.Date()
	daily := time.Date(y, m, d, dailyHour, dailyMinute, 0, 0, taipei.Location)
	last := time.Date(y, m, d, lastHour, lastMinute, 0, 0, taipei.Location)
	tomorrow := time.Date(y, m, d+1, dailyHour, dailyMinute, 0, 0, taipei.Location)

	switch {
	case local.Before(daily):
		return daily
	case !local.Before(last), haveToday:
		return tomorrow
	}
	retry := local.Add(retryInterval)
	if retry.After(last) {
		return last
	}
	return retry
}

type updateRunner interface {
	Update(ctx context.Context) (UpdateResult, error)
}

type Updater struct {
	svc updateRunner
	log *slog.Logger
	now func() time.Time
}

func NewUpdater(svc updateRunner, log *slog.Logger) *Updater {
	return &Updater{svc: svc, log: log, now: time.Now}
}

func (u *Updater) Run(ctx context.Context) {
	for {
		haveToday := u.runOnce(ctx)
		timer := time.NewTimer(nextRun(u.now(), haveToday).Sub(u.now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (u *Updater) runOnce(ctx context.Context) (haveToday bool) {
	start := time.Now()
	defer func() {
		if p := recover(); p != nil {
			u.log.ErrorContext(ctx, "rates update panicked", "job", "rates", "panic", p, "stack", string(debug.Stack()))
		}
	}()

	res, err := u.svc.Update(ctx)
	if err != nil {
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			u.log.InfoContext(ctx, "rates update cancelled", "job", "rates")
			return false
		}
		u.log.ErrorContext(ctx, "rates update failed", "job", "rates", "err", err)
		return false
	}
	u.log.InfoContext(ctx, "rates updated", "job", "rates", "source", res.Source,
		"fetched", res.Fetched, "inserted", res.Inserted, "updated", res.Updated,
		"latest_date", res.Latest.Format(time.DateOnly), "duration_ms", time.Since(start).Milliseconds())
	return res.Latest.Equal(taipei.Today(u.now()))
}
