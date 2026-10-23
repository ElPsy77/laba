# Лабораторная работа № 5. Веб-приложение на Spring MVC (Go-версия)

**Студент:** Каримбаев Макжан, группа 2502IS · **Вариант:** 1 (поиск по фрагменту названия) · **Ветка:** `lab05`

## 1. Цель работы

Построить работающий веб-слой: объявить контроллеры обоих видов, сопоставить URL с методами, прочитать все части HTTP-запроса в параметры метода, возвращать осмысленные коды статуса через ResponseEntity, передавать модель в представление и раздавать статические ресурсы. Отдельная цель — по коду статуса понимать, какая именно часть сопоставления не совпала.

## 2. Как Spring MVC устроен в Go-версии

| Spring MVC | Go (`internal/app`, `internal/web`) |
|---|---|
| `DispatcherServlet` | `app.NewRouter()` — единый `http.ServeMux`, через который проходит каждый запрос |
| `HandlerMapping` (`@RequestMapping`, `@GetMapping`…) | шаблоны `ServeMux`: `"GET /api/books/{id}"` — метод + путь |
| `HandlerAdapter` для `@RestController` + `HttpMessageConverter` (Jackson) | хендлер пишет объект в тело через `app.WriteJSON` (`encoding/json`) |
| `HandlerAdapter` для `@Controller` + `ViewResolver` | `app.Page(...)`: создаёт `Model`, вызывает метод, имя представления `"books/list"` → `resources/templates/books/list.html` |
| Thymeleaf (`th:text`, `th:each`, `th:if`) | `html/template` (`{{.title}}`, `{{range .books}}`, `{{with .message}}`) — тоже экранирует HTML |
| `@PathVariable long id` | `r.PathValue("id")` + `strconv.ParseInt`; не число → 400 |
| `@RequestParam(required = false)` / `(defaultValue = "10")` | `r.URL.Query().Get(...)`; пусто → `""` (null) / значение по умолчанию |
| `@RequestBody Book` | `app.CheckJSONBody` (Content-Type → 415) + `json.Decoder` (кривой JSON → 400) |
| content negotiation по `Accept` | `app.CheckAcceptsJSON`: клиент принимает только XML → 406 |
| `ResponseEntity.ok / notFound / created / noContent` | `WriteJSON(200)`, `WriteHeader(404)`, `Location` + `WriteJSON(201)`, `WriteHeader(204)` |
| `"redirect:/books"` | `app.Page` отвечает `302 Found` + `Location: /books` |
| `RedirectAttributes.addFlashAttribute` + `FlashMapManager` в HTTP-сессии | flash-карта хранится на сервере по cookie `JSESSIONID` и удаляется после первого использования |
| `ResourceHttpRequestHandler` для `classpath:/static/**`, welcome page `index.html` | fallback-хендлер роутера отдаёт файлы из `resources/static/`, `/` → `index.html` |

Обработка запроса, на который не нашлось контроллера, повторяет порядок Spring Boot: сначала статический ресурс, затем проверка «путь есть, но для другого метода» (405 + заголовок `Allow`), и только потом 404.

## 3. Задание 1. Домен, репозиторий, сервис

Пакет `internal/catalog` (аналог `kz.iitu.springlab.catalog`):

- `book.go` — `Book{ID, Title, Author, Year}`. Структура передаётся по значению, поэтому неизменяема, как record. JSON-теги играют роль Jackson. `ID == 0` означает Java-шный `null`, то есть «ещё не сохранена».
- `book_repository.go` — `BookRepository` (`@Repository`). `map` под `sync.RWMutex` заменяет `ConcurrentHashMap`, `atomic.Int64` заменяет `AtomicLong`. При создании репозиторий содержит три книги.
- `book_service.go` — `BookService` (`@Service`) получает репозиторий через конструктор. `FindAll("")` возвращает всё, `FindAll("Bloch")` фильтрует по фрагменту автора без учёта регистра. Ещё есть `FindByID`, `Create`, `Replace`, `Delete`.

**Контрольная точка 1:** приложение стартует без ошибок. Любой URL без контроллера отвечает JSON-ом `{"status":404,"error":"Not Found",...}` — это аналог whitelabel-страницы, и он доказывает, что «DispatcherServlet» работает.

## 4. Задание 2. REST-контроллер

`internal/web/book_rest_controller.go`:

```go
func (c *BookRestController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/books", c.list)
	mux.HandleFunc("GET /api/books/{id}", c.find)
	mux.HandleFunc("POST /api/books", c.create)
	mux.HandleFunc("PUT /api/books/{id}", c.replace)    // TODO 2.2
	mux.HandleFunc("DELETE /api/books/{id}", c.delete)  // TODO 2.3
	mux.HandleFunc("GET /api/books/search", c.search)   // вариант 1
}
```

Откуда берётся каждый параметр:

| Параметр | Часть запроса | Аналог в Spring |
|---|---|---|
| `id` в `/api/books/{id}` | сегмент пути | `@PathVariable long id` |
| `author`, `limit` | строка запроса | `@RequestParam(required = false) String author`, `@RequestParam(defaultValue = "10") int limit` |
| тело книги в POST/PUT | тело запроса | `@RequestBody Book book` |
| `Content-Type`, `Accept` | заголовки | используются конвертерами сообщений |

**Результаты (Листинг 3, задание 2.4):**

| Запрос | Статус | Тело / заголовки |
|---|---|---|
| `GET /api/books` | **200** | `[{"id":1,"title":"Effective Java",...},{"id":2,...},{"id":3,...}]` |
| `GET /api/books?author=Bloch&limit=1` | **200** | `[{"id":1,"title":"Effective Java","author":"Joshua Bloch","year":2018}]` |
| `GET /api/books/1` | **200** | `{"id":1,"title":"Effective Java","author":"Joshua Bloch","year":2018}` |
| `GET /api/books/999` | **404** | пустое тело (`ResponseEntity.notFound().build()`) |
| `POST /api/books` + JSON | **201** | `Location: /api/books/4`, `{"id":4,"title":"Pro Spring 6","author":"Cosmina","year":2023}` |
| `PUT /api/books/4` + JSON | **200** | `{"id":4,"title":"Pro Spring 6","author":"Iuliana Cosmina","year":2023}` |
| `PUT /api/books/999` + JSON | **404** | пустое тело |
| `DELETE /api/books/2` (1-й раз) | **204** | пустое тело |
| `DELETE /api/books/2` (2-й раз) | **404** | пустое тело |

- **Почему 201, а не 200:** создан новый ресурс. В заголовке `Location` его адрес, и GET по этому адресу сразу возвращает созданную книгу.
- **Почему второй DELETE даёт 404:** после первого вызова книги №2 больше нет, удалять нечего. Отсутствие ресурса сообщается через `ResponseEntity`, без исключения.

## 5. Задание 3. Страница, модель, статика

`internal/web/book_page_controller.go` — обычный контроллер (`@Controller`). Он ничего не пишет в тело ответа сам: заполняет модель и возвращает имя представления.

```go
func (c *BookPageController) list(r *http.Request, model app.Model, _ *app.RedirectAttributes) (string, error) {
	author := r.URL.Query().Get("author")
	model.AddAttribute("books", c.service.FindAll(author))
	model.AddAttribute("author", author)
	model.AddAttribute("title", "Catalogue")
	return "books/list", nil // -> templates/books/list.html
}
```

- Модель **заполняет** контроллер, а **читает** шаблон `resources/templates/books/list.html` через `{{.title}}`, `{{range .books}}` и `{{with .message}}`.
- Имя `books/list` превращает в файл `app.render` (аналог `ThymeleafViewResolver`): префикс `templates/` + имя + суффикс `.html`.
- `/css/app.css` и `/` работают без контроллера. Ни один шаблон их не сопоставляет, поэтому запрос доходит до обработчика статических ресурсов, и он отдаёт файлы из `resources/static/` (`css/app.css`, `index.html`). Путь очищается через `path.Clean`, поэтому `..` не позволит выйти за пределы `static/`.

**Контрольная точка 3:**

```
GET /books              -> 200 text/html, таблица из 3 книг, стили применены
GET /books?author=Bloch -> 200, в таблице только «Effective Java»
GET /css/app.css        -> 200 text/css
GET /                   -> 200 text/html (static/index.html со ссылкой на /books)
```

> Скриншоты: `/books` и `/books?author=Bloch` (вставить в PDF-отчёт).

**Что изменится при замене шаблонизатора:** контроллеры не изменятся вообще. Они возвращают только имя представления и модель. Поменяются шаблоны и `ViewResolver` (`app.render`).

## 6. Задание 4. Post/Redirect/Get и коды статуса

```go
func (c *BookPageController) create(r *http.Request, _ app.Model, redirect *app.RedirectAttributes) (string, error) {
	// @RequestParam String title, author; @RequestParam int year
	saved := c.service.Create(catalog.Book{Title: ..., Author: ..., Year: year})
	redirect.AddFlashAttribute("message", "Book «"+saved.Title+"» has been added")
	return "redirect:/books", nil // 302; the browser performs a GET
}
```

```
$ curl -i -X POST http://localhost:8080/books -d "title=Java Precisely&author=Sestoft&year=2016"
HTTP/1.1 302 Found
Location: /books
Set-Cookie: JSESSIONID=02C95F58C6A2B5CB39C7F92851A7BA0D; Path=/; HttpOnly
```

- Браузер делает **два запроса**: `POST /books` → **302** и `GET /books` → **200**.
- Flash-сообщение лежит **на сервере в сессии** (ключ — cookie `JSESSIONID`) между этими двумя запросами. Его **забирает и удаляет** `app.Page` при первом `GET /books`. Поэтому после F5 сообщения уже нет.
- **F5 не создаёт дубликат:** браузер повторяет последний запрос, а последним был `GET /books`, а не `POST`.

**Таблица кодов статуса (4.3):**

| Что отправляем | Статус | Почему именно этот статус |
|---|---|---|
| `GET /api/books/abc` | **400** | Путь и метод совпали с `GET /api/books/{id}`, но `abc` не преобразуется в `long`. Ошибка возникает до вызова логики метода (`MethodArgumentTypeMismatchException`). |
| `GET /api/nothing` | **404** | Не совпал **путь**: такой шаблон не зарегистрирован, статического файла с таким именем тоже нет. |
| `DELETE /api/books` (без id) | **405** | Путь `/api/books` есть, но только для GET и POST. Не совпал **метод**, в ответе заголовок `Allow: GET, POST`. |
| `POST /api/books` с `Content-Type: text/plain` | **415** | Путь и метод совпали, но для `@RequestBody` нет конвертера, читающего `text/plain` в `Book`. Не совпал **тип входящего тела**. |
| `GET /api/books` с `Accept: application/xml` | **406** | Путь и метод совпали, но клиент принимает только XML, а ответ умеем писать только в JSON. Не совпал **тип ответа**. |

415 и 406 различаются направлением. **415** — сервер не понимает то, что **прислал клиент** (`Content-Type`). **406** — сервер не может выдать то, что **клиент хочет получить** (`Accept`).

Дополнительно: `GET /api/books?limit=abc` → **400** (`limit` не число). Если параметр не передан совсем, берётся значение по умолчанию 10.

## 7. Индивидуальное задание — вариант 1

`GET /api/books/search?title=…` ищет по фрагменту названия без учёта регистра. Пустой результат — это успешный поиск, поэтому ответ **200 с пустым массивом**, а не 404.

```go
func (s *BookService) SearchByTitle(fragment string) []Book {
	return filter(s.repository.FindAll(), func(b Book) bool { return containsFold(b.Title, fragment) })
}
```

`filter` создаёт срез через `make(..., 0, n)`, поэтому в JSON уходит `[]`, а не `null`. Шаблон `GET /api/books/search` более конкретный, чем `GET /api/books/{id}`, поэтому `search` не принимается за id.

| Запрос | Статус | Ответ |
|---|---|---|
| `/api/books/search?title=java` | 200 | `[{"id":1,"title":"Effective Java",...}]` |
| `/api/books/search?title=CLEAN` | 200 | `[{"id":2,"title":"Clean Code",...}]` |
| `/api/books/search?title=python` | 200 | `[]` |
| `/api/books/search` (без title) | 400 | `Required request parameter 'title' ... is not present` |

## 8. Автотесты

`internal/web/book_controller_test.go` проверяет всё перечисленное выше: пять REST-операций, таблицу 400/404/405/406/415, рендеринг страницы с фильтром, статику, PRG (302, flash показывается один раз, F5 не создаёт дубликат) и вариант 1.

```
$ make test
ok  	springlab/internal/web
```

## 9. Выводы

Шаблон «front controller» даёт приложению одну точку входа для всех запросов. Сопоставление URL, разбор параметров, преобразование тела, выбор формата ответа и ошибки 400/404/405/406/415 решаются в одном месте, а не в каждом обработчике. Поэтому методы контроллеров остаются короткими и содержат только логику: прочитать параметры, вызвать сервис, вернуть статус или имя представления. Разделение `@RestController` (объект → JSON) и `@Controller` (модель → шаблон) позволяет держать API и страницы в одном приложении над одним и тем же сервисом. Post/Redirect/Get с flash-атрибутами защищает форму от повторной отправки при обновлении страницы.
