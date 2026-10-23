package catalog

import (
	"strings"

	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewBookService) // @Service
}

// BookService takes the repository through its constructor (task 1.4).
type BookService struct {
	repository *BookRepository
}

func NewBookService(repository *BookRepository) *BookService {
	return &BookService{repository: repository}
}

// FindAll returns every book when author is empty (Java: null) and filters
// by author otherwise. "Bloch" finds "Joshua Bloch": a case-insensitive
// fragment match.
func (s *BookService) FindAll(author string) []Book {
	all := s.repository.FindAll()
	if author == "" {
		return all
	}
	return filter(all, func(b Book) bool { return containsFold(b.Author, author) })
}

func (s *BookService) FindByID(id int64) (Book, bool) {
	return s.repository.FindByID(id)
}

// Create always makes a new record: an id sent by the client is ignored.
func (s *BookService) Create(book Book) Book {
	book.ID = 0
	return s.repository.Save(book)
}

// Replace overwrites an existing record; false means there was nothing to replace.
func (s *BookService) Replace(id int64, book Book) (Book, bool) {
	if _, ok := s.repository.FindByID(id); !ok {
		return Book{}, false
	}
	book.ID = id
	return s.repository.Save(book), true
}

func (s *BookService) Delete(id int64) bool {
	return s.repository.DeleteByID(id)
}

// SearchByTitle — individual assignment, variant 1: a case-insensitive
// search by a fragment of the title. Never nil, so JSON gives [] and not null.
func (s *BookService) SearchByTitle(fragment string) []Book {
	return filter(s.repository.FindAll(), func(b Book) bool { return containsFold(b.Title, fragment) })
}

func filter(books []Book, keep func(Book) bool) []Book {
	out := make([]Book, 0, len(books))
	for _, b := range books {
		if keep(b) {
			out = append(out, b)
		}
	}
	return out
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
