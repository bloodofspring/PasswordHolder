package models

type Users struct {
	ID        int64  `pg:"id,pk"`
	CreatedAt int64 `pg:",default:extract(epoch from now())"`
	UpdatedAt int64 `pg:",default:extract(epoch from now())"`

	TelegramID int64  `pg:"telegram_id,pk"`
}
