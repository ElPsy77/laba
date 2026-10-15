# Лабораторная работа № 1. Первое веб-приложение (Go-версия)

**Дисциплина:** RWPSF 3305 — Web Application Development with Spring Framework
**Студент:** Каримбаев Макжан, группа 2502IS · **Вариант:** 1 · **Дата:** 15.10.2026
**Ветка:** `lab01`

## 1. Цель работы

Подготовить рабочее окружение, создать, запустить и опубликовать в Git первое веб-приложение, разобраться в структуре проекта и реализовать простой REST-эндпоинт. По требованию проекта все задания выполняются на **Go**, повторяя структуру и поведение Spring Boot-примеров.

## 2. Соответствие Spring Boot → Go

| Spring Boot | Go-реализация в проекте |
|---|---|
| JDK 25, `java -version` | Go 1.24, `go version` |
| Maven, `pom.xml`, Maven Wrapper | Go modules, `go.mod`, `Makefile` (`make run`, `make build`) |
| Spring Initializr | `go mod init springlab` + каркас `cmd/springlab/main.go` |
| `SpringLabApplication` + `@SpringBootApplication` | `cmd/springlab/main.go` |
| Встроенный Tomcat | `net/http` из стандартной библиотеки |
| `@RestController`, `@GetMapping` | `HelloController` + `mux.HandleFunc("GET /api/hello", …)` |
| `@RequestParam(defaultValue = "world")` | `r.URL.Query().Get("name")` с проверкой на пустую строку |
| `@Value("${app.owner:unknown}")` | `props.Get("app.owner", "unknown")` |
| Jackson (record → JSON) | `encoding/json` + теги `json:"…"` |
| Whitelabel Error Page | JSON-ответ того же формата: `timestamp`, `status`, `error`, `path` |
| `java -jar target/…jar` | `./target/spring-lab-01` (один исполняемый файл) |

## 3. Проверка окружения (задание 1)

```
$ go version
go version go1.24.7 linux/amd64
$ git --version
git version 2.43.0
$ git config --global user.name "Makzhan Karimbayev"
```

## 4. Создание проекта (задание 2)

| Параметр | Значение | Обоснование |
|---|---|---|
| Система сборки | Go modules | Встроенное управление зависимостями, аналог Maven |
| Модуль | `springlab` | Корневой «пакет» приложения (аналог `kz.iitu.springlab`) |
| Артефакт | `target/spring-lab-01` | Имя исполняемого файла, как имя JAR |
| Версия Go | 1.24 | Текущая стабильная версия на рабочем месте |
| Зависимости | только стандартная библиотека | Веб-стек уже есть в `net/http` |

## 5. Первый запуск (задание 3)

```
$ make run
2026/10/15 14:30:52 INFO Starting SpringLabApplication using Go go1.24.7 with PID 3610
2026/10/15 14:30:52 INFO HTTP server initialized with port 8080 (http)
2026/10/15 14:30:52 INFO Started SpringLabApplication in 0.000 seconds
```

Три строки для отчёта: версия платформы — `go1.24.7`; порт — `8080`; время старта — меньше 1 мс (Go компилируется в машинный код и не поднимает контейнер, поэтому старт мгновенный в отличие от ~1,8 с у Spring Boot).

**Контрольная точка.** Запрос к `/` до появления контроллера:

```
HTTP/1.1 404 Not Found
{"error":"Not Found","path":"/","status":404,"timestamp":"2026-10-15T14:30:19.926"}
```

404 — ожидаемый результат: сервер работает, но обработчик для `/` не объявлен.

## 6. Структура проекта (задание 4)

| Путь | Назначение |
|---|---|
| `go.mod` | Описание модуля и зависимостей (аналог `pom.xml`) |
| `Makefile` | Команды сборки и запуска (аналог Maven Wrapper) |
| `cmd/springlab/` | Стартовый пакет `main` |
| `internal/` | Исходный код приложения, недоступный другим модулям |
| `resources/application.properties` | Настройки приложения |
| `resources/static/`, `resources/templates/` | Статика и шаблоны (пока пустые) |
| `*_test.go` | Автоматические тесты (лежат рядом с кодом) |
| `target/` | Результат сборки, не входит в Git |

Почему в проекте не нужны «транзитивные» библиотеки вроде `spring-webmvc`, `tomcat-embed-core`, `jackson-databind`: их роль в Go выполняет стандартная библиотека (`net/http`, `encoding/json`), поэтому `go.mod` не содержит ни одной внешней зависимости.

## 7. REST-контроллер (задание 5)

Листинг — `internal/web/hello_controller.go`. Настройки — `resources/application.properties`:

```properties
spring.application.name=spring-lab-01
server.port=8080
# put your own surname and group here
app.owner=Karimbayev Makzhan, 2502IS
logging.level.web=INFO
```

## 8. Проверка эндпоинтов (задание 6)

```
GET /api/hello            -> 200 {"message":"Hello, world!","owner":"Karimbayev Makzhan, 2502IS","timestamp":"2026-10-15T14:30:53.970"}
GET /api/hello?name=IITU  -> 200 {"message":"Hello, IITU!","owner":"Karimbayev Makzhan, 2502IS","timestamp":"2026-10-15T14:30:53.976"}
GET /api/info             -> 200 {"owner":"Karimbayev Makzhan, 2502IS","goVersion":"go1.24.7","cpuCores":2,"pid":3610}
GET /api/none             -> 404 {"error":"Not Found","path":"/api/none","status":404,...}
```

Смена порта на 9090 — строка лога, подтверждающая изменение:

```
INFO HTTP server initialized with port 9090 (http)
```

Если порт занят другим экземпляром:

```
ERROR Web server failed to start error="listen tcp :8080: bind: address already in use"
```

`/api/none` вернул **404**, потому что ни один обработчик не зарегистрирован на этот путь — срабатывает общий обработчик `/`.

## 9. Индивидуальное задание — вариант 1: `GET /api/sum?a=&b=`

Возвращает сумму, разность и произведение двух целых чисел; при отсутствии или ошибке параметра — 400.

| Запрос | Ответ |
|---|---|
| `/api/sum?a=7&b=3` | `{"a":7,"b":3,"sum":10,"difference":4,"product":21}` |
| `/api/sum?a=-5&b=12` | `{"a":-5,"b":12,"sum":7,"difference":-17,"product":-60}` |
| `/api/sum?a=0&b=0` | `{"a":0,"b":0,"sum":0,"difference":0,"product":0}` |
| `/api/sum?a=1` | `400 Bad Request: parameters a and b must be integers` |

Те же наборы проверяются автотестами `go test ./...` (`TestSumThreeDataSets`, `TestSumBadRequest`).

## 10. Git (задание 7)

```
git checkout -b lab01
git commit -m "lab01: add REST controller with /api/hello, /api/info and /api/sum"
git push -u origin lab01      # затем pull request в main
```

## 11. Выводы

Создано и запущено первое веб-приложение на Go. Роль Spring Boot (встроенный сервер, преобразование в JSON, чтение настроек) здесь выполняет стандартная библиотека, а «конвенция вместо конфигурации» заменена явным кодом: маршруты регистрируются вызовами `HandleFunc`, а параметры читаются из `application.properties`. Основная сложность — повторить формат ошибок Spring (Whitelabel), она решена общим `WriteError`.
