package common

import "fmt"

// Page is one page of a listing. TotalItems counts the items of every page.
type Page[T any] struct {
	Items      []T
	Number     int
	TotalPages int
	TotalItems int
}

// PageNotFoundError is what Paginate returns for a page past the last one.
// Noun names what the pages hold in the message, and stays empty to leave it
// out.
type PageNotFoundError struct {
	Number     int
	TotalPages int
	Noun       string
}

func (err *PageNotFoundError) Error() string {
	count := fmt.Sprintf("are %d pages", err.TotalPages)
	if err.TotalPages == 1 {
		count = "is 1 page"
	}
	if err.Noun != "" {
		count += " of " + err.Noun
	}
	return fmt.Sprintf("page %d does not exist, there %s", err.Number, count)
}

// Paginate returns the page of items numbered page, counting from 1, with
// size items on every page. A size of 0 puts every item on page 1. An empty
// list has one empty page. It refuses a page below 1 and a negative size, and
// returns a *PageNotFoundError for a page past the last one.
func Paginate[T any](items []T, page, size int) (Page[T], error) {
	if page < 1 {
		return Page[T]{}, fmt.Errorf("page must be 1 or more, got %d", page)
	}
	if size < 0 {
		return Page[T]{}, fmt.Errorf("page size must be 0 or more, got %d", size)
	}

	totalPages := 1
	if size > 0 && len(items) > 0 {
		totalPages = (len(items) + size - 1) / size
	}
	if page > totalPages {
		return Page[T]{}, &PageNotFoundError{Number: page, TotalPages: totalPages}
	}

	start, end := 0, len(items)
	if size > 0 {
		start = (page - 1) * size
		end = min(start+size, len(items))
	}
	return Page[T]{
		Items:      items[start:end],
		Number:     page,
		TotalPages: totalPages,
		TotalItems: len(items),
	}, nil
}
