// internal/models/filter.go
package repo

import "time"

type MovementFilter struct {
	Since  *time.Time
	Until  *time.Time
	Offset *int
	Limit  *int
}
