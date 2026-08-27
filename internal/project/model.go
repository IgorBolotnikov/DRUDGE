// Package project provides a project
package project

import "time"

type Project struct {
	Slug     string
	Name     string
	Location string

	CreatedAt time.Time
}
