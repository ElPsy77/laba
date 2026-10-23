package catalog

import (
	"sort"
	"sync"
	"sync/atomic"

	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewBookRepository) // @Repository
}

// BookRepository keeps the data in memory. A map behind a RWMutex is the Go
// counterpart of ConcurrentHashMap, atomic.Int64 of AtomicLong: the bean is a
// singleton and is called from many request goroutines at once.
type BookRepository struct {
	mu       sync.RWMutex
	storage  map[int64]Book
	sequence atomic.Int64
}

// NewBookRepository starts with three records.
func NewBookRepository() *BookRepository {
	r := &BookRepository{storage: map[int64]Book{}}
	r.Save(Book{Title: "Effective Java", Author: "Joshua Bloch", Year: 2018})
	r.Save(Book{Title: "Clean Code", Author: "Robert Martin", Year: 2008})
	r.Save(Book{Title: "Spring in Action", Author: "Craig Walls", Year: 2022})
	return r
}

// FindAll returns a copy (List.copyOf), ordered by id.
func (r *BookRepository) FindAll() []Book {
	r.mu.RLock()
	defer r.mu.RUnlock()
	books := make([]Book, 0, len(r.storage))
	for _, b := range r.storage {
		books = append(books, b)
	}
	sort.Slice(books, func(i, j int) bool { return books[i].ID < books[j].ID })
	return books
}

// FindByID is Optional<Book>: the second result says whether it is present.
func (r *BookRepository) FindByID(id int64) (Book, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.storage[id]
	return b, ok
}

// Save assigns the next id to a new record and stores it.
func (r *BookRepository) Save(book Book) Book {
	id := book.ID
	if id == 0 {
		id = r.sequence.Add(1)
	}
	stored := Book{ID: id, Title: book.Title, Author: book.Author, Year: book.Year}
	r.mu.Lock()
	r.storage[id] = stored
	r.mu.Unlock()
	return stored
}

// DeleteByID reports whether something was removed.
func (r *BookRepository) DeleteByID(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.storage[id]; !ok {
		return false
	}
	delete(r.storage, id)
	return true
}
