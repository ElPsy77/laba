# spring-lab-01 — Go edition

Лабораторные работы 1–5 курса **RWPSF 3305 — Web Application Development with Spring Framework**, выполненные на Go по заданиям и примерам лекций.

**Студент:** Каримбаев Макжан, группа 2502IS

| Лаба | Тема | Ветка | Отчёт |
|---|---|---|---|
| 1 | Первое веб-приложение, REST-контроллер | `lab01` | [docs/lab01/REPORT.md](docs/lab01/REPORT.md) |
| 2 | IoC-контейнер, DI, scopes, жизненный цикл | `lab02` | [docs/lab02/REPORT.md](docs/lab02/REPORT.md) |
| 3 | YAML, профили, источники свойств, валидация, actuator | `lab03` | [docs/lab03/REPORT.md](docs/lab03/REPORT.md) |
| 4 | AOP: аспекты, pointcut'ы, прокси, self-invocation | `lab04` | [docs/lab04/REPORT.md](docs/lab04/REPORT.md) |
| 5 | Spring MVC: REST и MVC-контроллеры, модель, шаблон, статика, PRG | `lab05` | [docs/lab05/REPORT.md](docs/lab05/REPORT.md) |

Каждая лаба влита в `main` merge-коммитом (вместо pull request).

## Запуск

Нужен Go 1.24+. Внешних зависимостей нет.

```bash
make run                                        # ./mvnw spring-boot:run
make run ARGS="--spring.profiles.active=dev"    # профиль dev (порт 8081)
make build && ./target/spring-lab-01 --debug    # сборка + отчёт об автоконфигурации
make test                                       # все автотесты
go generate ./...                               # перегенерировать AOP-прокси после правки аннотаций
```

## Устройство

```
cmd/springlab/        стартовый пакет (SpringLabApplication)
cmd/aopgen/           генератор AOP-прокси (аналог CGLIB, работает при сборке)
pkg/ioc/              IoC-контейнер (ApplicationContext): бины, DI, scopes, условия, BeanPostProcessor
pkg/env/              Environment: источники свойств, YAML, профили, привязка и валидация
pkg/aop/              Spring AOP: pointcut'ы AspectJ, 5 типов advice, @Order, AutoProxyCreator
internal/app/         DispatcherServlet: роутер, статика, 405, Model/ViewResolver, redirect и flash
internal/...          код лабораторных: web, catalog, notify, config, lifecycle, scope, service, aspect, audit, autoconfigure
resources/            application.yml, профильные файлы, templates/ и static/
docs/labNN/           отчёты
```

## Индивидуальные варианты

Номер варианта в задании зависит от номера в журнале/списке группы. Везде выполнен **вариант 1**; если номер другой, замените:

- Lab 1 — `Sum` в `internal/web/hello_controller.go`
- Lab 2 — `internal/notify/upper_notifier.go`
- Lab 3 — структура `UI` в `internal/config/app_properties.go` и ключи `app.ui` в YAML
- Lab 4 — `internal/aspect/call_counter_aspect.go`
- Lab 5 — `search` в `internal/web/book_rest_controller.go` и `SearchByTitle` в `internal/catalog/book_service.go`
