# Лабораторная работа № 2. IoC-контейнер и внедрение зависимостей (Go-версия)

**Студент:** Каримбаев Макжан, группа 2502IS · **Вариант:** 1 (`upper`) · **Ветка:** `lab02`

## 1. Цель работы

Освоить контейнер инверсии управления: объявлять бины разными способами, внедрять зависимости через конструктор, управлять областью видимости и жизненным циклом бина, разрешать неоднозначность при нескольких реализациях интерфейса.

## 2. Как устроен контейнер в Go-версии

В Go нет Spring и нет аннотаций, поэтому в проекте написан собственный небольшой контейнер `pkg/ioc` — упрощённый `ApplicationContext`. Он работает по тем же правилам, что описаны в лекции 2:

| Spring | Go (`pkg/ioc`) |
|---|---|
| `@Component("console")` + сканирование пакетов | `ioc.Component(NewConsoleNotifier, ioc.Name("console"))` в `init()` пакета; пакет участвует, только если импортирован в `main` — это и есть «область сканирования» |
| `@Bean`-метод в `@Configuration` | `ioc.Bean(ReportDateFormatter)`; имя бина = имя функции |
| `@Primary`, `@Fallback`, `@Order(n)` | опции `ioc.Primary()`, `ioc.Fallback()`, `ioc.Order(n)` |
| `@Qualifier("console")` у параметра | `ioc.Qualifier(1, "console")` — номер параметра конструктора |
| `List<Notifier>`, `Map<String, Notifier>` | параметры `[]Notifier`, `map[string]Notifier` |
| `@Scope("prototype")`, `ObjectProvider<T>` | `ioc.ScopePrototype()`, `*ioc.Provider[T]` |
| `@PostConstruct` / `@PreDestroy` | методы `PostConstruct()` / `PreDestroy()` |
| `BeanDefinition` | `ioc.Definition` — «рецепт» бина: имя, тип, фабрика, scope, флаги |
| `ApplicationContext`, `CommandLineRunner` | `*ioc.Container` (внедряется как обычная зависимость), `ioc.CommandLineRunner` |

Контейнер сначала читает **все** определения бинов (`ioc.New`), и только потом создаёт объекты (`Refresh`), поэтому порядок объявления не важен, а ошибки конфигурации видны при старте. Поведение контейнера закреплено тестами `pkg/ioc/container_test.go` (Primary/Fallback/Qualifier, порядок списка, prototype, обнаружение циклов).

## 3. Структура пакетов после работы

```
cmd/springlab/main.go          стартовый пакет, «сканирование» через импорты
pkg/ioc/                       контейнер
internal/notify/               Notifier, 3 реализации + вариант, NotificationService
internal/config/               FormatConfig (@Bean), ContainerReport (CommandLineRunner)
internal/lifecycle/            LifecycleDemo
internal/scope/                Ticket (prototype), TicketOffice (singleton)
internal/web/                  HelloController, Lab2Controller
```

## 4. Задание 2. Эксперимент с @Primary

После удаления `ioc.Primary()` у `email` приложение не стартует:

```
ERROR Application run failed error="UnsatisfiedDependencyException: Error creating bean with name
'notificationService' (component NewNotificationService): Unsatisfied dependency expressed through
constructor parameter 0: NoUniqueBeanDefinitionException: No qualifying bean of type 'notify.Notifier'
available: expected single matching bean but found 4: console,email,noop,upper"
```

**Почему @Fallback не спасает.** `@Fallback` лишь исключает `noop` из претендентов; выбор срабатывает, только если после этого остаётся **ровно один** обычный кандидат. Здесь остаются `console`, `email` и `upper` — неоднозначность сохраняется. Если бы реализаций было две (обычная + fallback), контейнер выбрал бы обычную — это проверяет тест `TestFallbackLeavesSingleRegularCandidate`.

## 5. Задание 3. Четыре точки внедрения

`GET /api/lab2/notify?text=IITU`:

```json
{"all":["console: IITU","email: IITU","upper: IITU","noop"],
 "beanNames":["console","email","noop","upper"],
 "console":"console: IITU","primary":"email: IITU"}
```

Порядок `all` — `console(1) → email(2) → upper(3) → noop(99)`. Его задаёт `@Order`: меньшее значение — раньше в списке; бины без порядка идут последними. `beanNames` — ключи `Map`, то есть имена бинов.

## 6. Задание 4. Бин через @Bean-метод

`ReportDateFormatter(props)` создаёт `*config.DateTimeFormatter` (аналог `java.time.format.DateTimeFormatter`) из шаблона `app.date-pattern=yyyy-MM-dd HH:mm:ss`. Имя бина — `reportDateFormatter`, по имени функции. Локаль и шаблон — это параметры создания, а не самостоятельные компоненты, поэтому отдельными бинами они не объявляются.

## 7. Задание 5. Жизненный цикл

```
INFO LIFECYCLE >> 2026-10-15 14:34:49  1. Constructor called
INFO LIFECYCLE >> 2026-10-15 14:34:49  2. @PostConstruct executed
INFO HTTP server initialized with port 8080 (http)
INFO Started SpringLabApplication in 0.000 seconds
...
INFO Shutting down: closing the application context
INFO LIFECYCLE >> 2026-10-15 14:34:50  3. @PreDestroy executed
```

Конструктор и `PostConstruct` выполняются **до** строки `Started` — синглтоны создаются при подъёме контекста. `PreDestroy` выполняется при штатной остановке (Ctrl+C / SIGTERM): `main` ловит сигнал, останавливает сервер и закрывает контейнер. При `kill -9` строк `PreDestroy` в логе 0: SIGKILL нельзя перехватить, процесс уничтожается ОС, код закрытия не выполняется.

## 8. Задание 6. Области видимости

| Что сравнивается | Первый запрос | Второй запрос |
|---|---|---|
| `injectedDirectly` — оба значения | `ddfb6830`, `ddfb6830` | `ddfb6830`, `ddfb6830` |
| `viaProvider` — оба значения | `27f54809`, `204b9ac1` | `42687ec3`, `1517c3ce` |
| `office` (идентичность синглтона) | `0xc00011c470` | `0xc00011c470` |

`injectedDirectly` не меняется: prototype-бин был создан **один раз** — при сборке синглтона `TicketOffice` — и сохранён в поле. `Provider.GetObject()` при каждом вызове снова обращается к контейнеру, и тот создаёт новый экземпляр prototype-бина.

## 9. Задание 7. Содержимое контейнера

```
INFO Bean definitions in total: 13
INFO Notifier implementations: [console email noop upper]
INFO Type of the notificationService bean: *notify.NotificationService
INFO   bean: notificationService    *notify.NotificationService
INFO   bean: console                *notify.ConsoleNotifier
INFO   bean: email                  *notify.emailNotifier
...
```

13 бинов = 12 компонентов приложения + `applicationProperties`. В Spring Boot их сотни, потому что автоконфигурация добавляет инфраструктурные бины; в Go-версии инфраструктуры нет, поэтому видно только своё.

## 10. Индивидуальное задание — вариант 1 (`upper`)

`internal/notify/upper_notifier.go`: имя `upper`, `@Order(3)`, `PostConstruct` пишет строку в лог, внедряется в контроллер через `ioc.Qualifier(3, "upper")`.

```
INFO UPPER >> upperNotifier initialised (@PostConstruct)
GET /api/lab2/custom?text=hello iitu       -> {"channel":"upper","result":"upper: HELLO IITU"}
GET /api/lab2/custom?text=Spring Boot 4    -> {"channel":"upper","result":"upper: SPRING BOOT 4"}
GET /api/lab2/custom?text=студент          -> {"channel":"upper","result":"upper: СТУДЕНТ"}
```

## 11. Ответы на ключевые вопросы защиты

- **Почему конструктор, а не поле?** Поля задаются один раз, объект всегда собран полностью, в тесте создаётся простым вызовом `NewX(...)` без контейнера. В Go внедрения в поле просто нет — только через конструктор.
- **Почему синглтон не должен хранить изменяемое состояние?** Один объект обслуживает параллельные запросы из разных горутин. `LifecycleDemo` хранит список событий, поэтому доступ к нему защищён `sync.Mutex`.
- **Циклические зависимости** обнаруживаются при старте: `BeanCurrentlyInCreationException ... Is there an unresolvable circular reference?` (тест `TestCircularDependencyIsDetected`).

## 12. Выводы

Принцип IoC реализован явно: объекты не создают свои зависимости сами, их собирает контейнер по описаниям. Написание собственного контейнера показало, откуда берутся ошибки Spring: `NoUniqueBeanDefinitionException` — это просто «нашлось больше одного кандидата», а `@Primary`/`@Fallback`/`@Qualifier` — правила выбора из них. Найденная при работе ошибка: тип настроек `map[string]string` контейнер сначала принял за Map-инъекцию; исправлено приоритетом точного совпадения типа.
