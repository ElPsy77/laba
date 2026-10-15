# Лабораторная работа № 4. Сквозная функциональность: AOP (Go-версия)

**Студент:** Каримбаев Макжан, группа 2502IS · **Вариант:** 1 (счётчик вызовов) · **Ветка:** `lab04`

## 1. Цель работы

Научиться выносить сквозную функциональность в отдельные модули: объявлять аспекты, писать pointcut-выражения на языке AspectJ, применять все пять типов advice, управлять порядком аспектов, привязывать advice к собственной аннотации, увидеть механизм проксирования и экспериментально подтвердить, что self-invocation не перехватывается.

## 2. Как AOP устроен в Go-версии

В Go нет ни CGLIB (генерации подклассов во время работы), ни динамических прокси, ни аннотаций. Поэтому механизм Spring AOP повторён так:

| Spring AOP | Go (`pkg/aop`, `cmd/aopgen`) |
|---|---|
| `spring-boot-starter-aop` | импорт пакета `springlab/pkg/aop` — он регистрирует `AopAutoConfiguration` |
| `@Aspect @Component @Order(n)` | тип с методом `Advices()` + `ioc.Component(..., ioc.Order(n))` |
| `@Pointcut` в классе `Pointcuts` | `aop.Library("springlab.internal.aspect.Pointcuts", aop.P(...))` |
| `@Before`, `@AfterReturning`, `@AfterThrowing`, `@After`, `@Around` | `aop.Before`, `aop.AfterReturning`, `aop.AfterThrowing`, `aop.After`, `aop.Around` |
| `JoinPoint`, `ProceedingJoinPoint.proceed()` | `*aop.JoinPoint`, `pjp.Proceed()` / `pjp.ProceedWith(args)` |
| `@annotation(audited)` + параметр `Audited audited` | `aop.AroundAnnotated("@annotation(audited)", func(pjp, audited audit.Audited) ...)` — привязка по типу параметра |
| Аннотация `@Audited` с `@Retention(RUNTIME)` | структура `audit.Audited` + комментарий `// @Audited(action = "...")`; генератор превращает его в значение времени выполнения |
| CGLIB-подкласс `CatalogService$$SpringCGLIB$$0` | сгенерированный тип `catalogServiceAopProxy` (`go generate`), реализующий интерфейс `CatalogService` |
| `AnnotationAwareAspectJAutoProxyCreator` (BeanPostProcessor) | `aop.AutoProxyCreator` — BeanPostProcessor контейнера; оборачивает бин **после** `PostConstruct`, только если хоть один pointcut совпал |

Язык pointcut тот же: `within(...)`, `execution(...)`, `@annotation(...)`, `bean(...)`, операторы `&&`, `||`, `!` и ссылки на именованные pointcut'ы. Пакеты пишутся через точку (`springlab.internal.service`), а «public» в Go — это экспортируемый метод (с заглавной буквы).

## 3. Задание 1. Сервис и контроллер

`internal/service/catalog_service.go` (интерфейс `CatalogService` + реализация `catalogService`), `internal/web/catalog_controller.go`. **Контрольная точка 1:**

```
GET    /api/lab4/item/7          -> {"result":"Item no. 7"}
GET    /api/lab4/items?limit=3   -> {"result":["Item no. 1","Item no. 2","Item no. 3"]}
DELETE /api/lab4/item/0          -> 500 {"error":"Internal Server Error","message":"Invalid identifier: 0",...}
```

Записей аспектов в логе нет. Прокси тоже не создаётся: `AutoProxyCreator` оборачивает бин, только если к нему применим хотя бы один advice.

## 4. Задание 2. Библиотека pointcut'ов и аспект логирования

```go
var Pointcuts = aop.Library("springlab.internal.aspect.Pointcuts",
	aop.P("serviceLayer", "within(springlab.internal.service..*)"),
	aop.P("publicMethod", "execution(public * *(..))"),
	aop.P("serviceOperation", "serviceLayer() && publicMethod()"),
)
```

- `within(springlab.internal.service..*)` — любой тип в пакете service **и его подпакетах**: `..` означает любое число пакетов, `*` — ровно один сегмент имени.
- `execution(public * *(..))` — выполнение любого публичного (экспортируемого) метода с любым возвращаемым типом и любыми параметрами.
- `serviceOperation` — пересечение обоих.

**Контрольная точка 2** (лог):

```
INFO  [LOG] -> CatalogService.FindByID(..) args=[7]
INFO  [LOG] <- FindByID returned Item no. 7
INFO  [LOG] -> CatalogService.FindAll(..) args=[3]
INFO  [LOG] <- FindAll returned [Item no. 1 Item no. 2 Item no. 3]
INFO  [LOG] -> CatalogService.Remove(..) args=[0]
ERROR [LOG] !! Remove threw *service.IllegalArgumentError: Invalid identifier: 0
```

Успешный вызов — два advice (`@Before` + `@AfterReturning`), неудачный — тоже два, но другие (`@Before` + `@AfterThrowing`).

**Эксперименты с типичными ошибками:**

| Ошибка | Результат |
|---|---|
| Аспект не зарегистрирован как компонент (нет `@Component`) | Ответ 200, записей `[LOG]` — **0**, ошибок — **0**: «код не работает, ошибок нет» |
| Ссылка на pointcut без полного имени: `"serviceOperation()"` | Отказ при старте: `IllegalArgumentException: LoggingAspect @Before("serviceOperation()"): error at ::18 can't find referenced pointcut serviceOperation` |

## 5. Задание 3. Замер времени, аудит, порядок аспектов

`TimingAspect` — `@Order(3)`, `LoggingAspect` — `@Order(2)`, `AuditAspect` — `@Order(1)`. Аннотированы `FindAll` (`CATALOG_LIST`, `logArguments = true`) и `Remove` (`CATALOG_REMOVE`); `FindByID` без аннотации.

```go
// @Audited(action = "CATALOG_LIST", logArguments = true)
func (s *catalogService) FindAll(limit int) ([]string, error) { ... }
```

После `go generate` в `catalogservice_aop.go` аннотация становится значением:

```go
{... Name: "FindAll", Annotations: []any{audit.Audited{Action: "CATALOG_LIST", LogArguments: true}}},
```

**`GET /api/lab4/items?limit=5` — порядок записей:**

| № | Запись в логе | Аспект и почему |
|---|---|---|
| 1 | `INFO [AUDIT] start CATALOG_LIST args=[5]` | AuditAspect, `@Order(1)` — наивысший приоритет, первый на входе |
| 2 | `INFO [LOG] -> CatalogService.FindAll(..) args=[5]` | LoggingAspect, `@Order(2)` |
| 3 | `WARN [TIME] SLOW: CatalogService.FindAll(..) — 300 ms` | TimingAspect, `@Order(3)` — последний на входе… |
| 4 | `INFO [LOG] <- FindAll returned [Item no. 1 ... Item no. 5]` | …и первый на выходе; обратный путь идёт в обратном порядке |
| 5 | `INFO [AUDIT] CATALOG_LIST success at 2026-10-15T14:46:38.936` | AuditAspect закрывает последним |

Порядок совпадает с объявленными `@Order`: чем меньше значение, тем выше приоритет — такой аспект внешний слой, первый на входе и последний на выходе.

`FindByID` логируется и замеряется, но **не** аудируется:

```
INFO [LOG] -> CatalogService.FindByID(..) args=[7]
INFO [TIME] CatalogService.FindByID(..) — 51 ms
INFO [LOG] <- FindByID returned Item no. 7
```

Вызов с ошибкой (`DELETE /api/lab4/item/0`) — ошибка не подавлена ни одним аспектом и дошла до клиента (500):

```
INFO  [AUDIT] start CATALOG_REMOVE
INFO  [LOG] -> CatalogService.Remove(..) args=[0]
INFO  [TIME] CatalogService.Remove(..) — 0 ms
ERROR [LOG] !! Remove threw *service.IllegalArgumentError: Invalid identifier: 0
WARN  [AUDIT] CATALOG_REMOVE failure at 2026-10-15T14:46:39.002: Invalid identifier: 0
```

**Эксперимент «@Around без proceed()»:** endpoint `/api/lab4/item/7` вернул `{"result":""}`, а в логе `[LOG] <- FindByID returned <nil>` — целевой метод не выполнялся вообще (нет 50 мс задержки, время 0 мс).

## 6. Задание 4. Прокси и self-invocation

`GET /api/lab4/proxy`:

```json
{"className":"*service.catalogServiceAopProxy","targetClass":"*service.catalogService",
 "isAopProxy":"true","isCglib":"false","isInterfaceProxy":"true"}
```

**Почему тип отличается от `catalogService`.** Контейнер внедрил в контроллер не сам объект, а сгенерированный прокси `catalogServiceAopProxy`. Суффикс `AopProxy` — аналог `$$SpringCGLIB$$0`: это сгенерированный класс-обёртка. Прокси хранит настоящий объект (`targetClass`) и каждый вызов пропускает через цепочку advice.

**Почему не CGLIB.** Spring Boot по умолчанию выбирает CGLIB, чтобы бин можно было внедрять и по классу, а не только по интерфейсу. В Go нельзя унаследоваться от структуры, поэтому возможен только вариант, аналогичный JDK-прокси: прокси реализует **интерфейс**. Следствие то же, что у JDK-прокси в Spring: запросить бин по конкретному типу нельзя — тест `TestBeanIsAProxyOfTheInterface` получает `BeanNotOfRequiredTypeException`.

**Таблица наблюдений (до исправления):**

| Вызов | Ожидалось записей аудита | Фактически |
|---|---|---|
| `DELETE /api/lab4/item/5` (внешний вызов `Remove`) | 2 (start + success) | **2** |
| `GET /api/lab4/remove-twice/5` (два self-invocation) | 0 | **0** |
| Сколько раз сработал LoggingAspect во втором случае | 2 (вход и выход `RemoveTwice`) | **2** |

Лог до исправления:

```
INFO [LOG] -> CatalogService.RemoveTwice(..) args=[5]
INFO [TIME] CatalogService.RemoveTwice(..) — 0 ms
INFO [LOG] <- RemoveTwice returned Removed item no. 5; Removed item no. 6
```

**Объяснение.** Advice сработали для `RemoveTwice`, потому что контроллер вызвал его через прокси. Внутри метода `s.Remove(id)` вызывается через получатель `s` — это сам объект `*catalogService`, а не прокси (то же, что `this` в Java). Вызов не проходит через прокси, и цепочка advice для него просто не запускается. По этой же причине в Spring `@Transactional` «не работает» при вызове из соседнего метода того же класса.

**Исправление — внедрение ссылки на себя** (аналог `@Autowired private CatalogService self`): `catalogService` реализует `aop.SelfAware`, и `AutoProxyCreator` после создания прокси помещает его в поле `self`; `RemoveTwice` вызывает `s.via().Remove(...)`. Выбран этот способ, а не вынесение `Remove` в отдельный бин, потому что он не меняет структуру сервиса и точно повторяет приём из лекции.

Лог после исправления:

```
INFO [LOG] -> CatalogService.RemoveTwice(..) args=[5]
INFO [AUDIT] start CATALOG_REMOVE
INFO [LOG] -> CatalogService.Remove(..) args=[5]
INFO [TIME] CatalogService.Remove(..) — 0 ms
INFO [LOG] <- Remove returned Removed item no. 5
INFO [AUDIT] CATALOG_REMOVE success at 2026-10-15T14:47:19.371
INFO [AUDIT] start CATALOG_REMOVE
INFO [LOG] -> CatalogService.Remove(..) args=[6]
INFO [TIME] CatalogService.Remove(..) — 0 ms
INFO [LOG] <- Remove returned Removed item no. 6
INFO [AUDIT] CATALOG_REMOVE success at 2026-10-15T14:47:19.371
INFO [TIME] CatalogService.RemoveTwice(..) — 0 ms
INFO [LOG] <- RemoveTwice returned Removed item no. 5; Removed item no. 6
```

Оба этапа есть в истории Git: коммит `lab04: proxy endpoint and self-invocation experiment` (до) и следующий коммит с исправлением.

## 7. Индивидуальное задание — вариант 1: счётчик вызовов

`internal/aspect/call_counter_aspect.go`: `@Before` на `Pointcuts.serviceLayer()`, счётчики по имени метода в `sync.Map` с `atomic.Int64` (аналог `ConcurrentHashMap`: аспект — синглтон, вызывается из многих горутин). Эндпоинт `GET /api/lab4/stats` после серии вызовов (2× FindByID, 1× FindAll, DELETE 0, remove-twice):

```json
[{"method":"CatalogService.FindAll(..)","calls":1},
 {"method":"CatalogService.FindByID(..)","calls":2},
 {"method":"CatalogService.Remove(..)","calls":3},
 {"method":"CatalogService.RemoveTwice(..)","calls":1}]
```

`Remove` = 3: один внешний DELETE и два вызова изнутри `RemoveTwice`, которые после исправления идут через прокси.

## 8. Автотесты

- `pkg/aop/aop_test.go` — разбор и сопоставление pointcut-выражений (`..` против `*`, `public`, `&&`/`||`/`!`), именованные ссылки, неподдерживаемый `call()`, фиксированный порядок advice внутри аспекта и вложенность аспектов, путь с ошибкой (`@AfterThrowing` вместо `@AfterReturning`, ошибка не подавлена).
- `internal/service/proxy_test.go` — бин является прокси интерфейса, по конкретному типу не выдаётся, self-invocation после исправления проходит через прокси, объект, созданный через `new`, не перехватывается.

## 9. Ответы на вопросы защиты (кратко)

- **Pointcut и advice в моём коде:** `Pointcuts` и строки-выражения отвечают «где»; методы `before`, `measure`, `audit` отвечают «что делать».
- **Зачем и `@Aspect`, и `@Component`?** Тип с `Advices()` только объявляет advice; бином его делает регистрация `ioc.Component`. Без неё аспект не найден — проверено экспериментом.
- **Порядок advice одного аспекта:** `@Around` (до `Proceed`) → `@Before` → метод → `@AfterReturning` или `@AfterThrowing` → `@After` → `@Around` (после `Proceed`) — закреплено тестом `TestOrderOfAdvice`.
- **Почему `@AfterReturning` не сработал на `DELETE /item/0`?** Метод завершился ошибкой, поэтому выполнился `@AfterThrowing`.
- **Зачем RUNTIME?** Комментарий `// @Audited(...)` существует только в исходнике (SOURCE). Чтобы advice получил экземпляр аннотации, она должна попасть в метаданные во время выполнения — это делает `go generate`. Если забыть перегенерировать прокси, `@annotation(...)` ничего не найдёт — ровно как при `Retention` по умолчанию.
- **Сработает ли аспект на приватном/статическом методе?** Нет: неэкспортируемые методы и функции пакета не входят в интерфейс и не проходят через прокси, а объект, созданный через `new`, не является бином.
- **Три аннотации Spring на том же механизме:** `@Transactional`, `@Cacheable`, `@Async` (а также `@PreAuthorize`, `@Retryable`).

## 10. Выводы

Логирование, замер времени и аудит вынесены из бизнес-кода в аспекты: сервис содержит только полезную работу, а формат записей меняется в одном месте. Повторив механизм на Go, стало видно, что все ограничения Spring AOP следуют из проксирования: перехватываются только вызовы через прокси, поэтому self-invocation, приватные методы и объекты, созданные вручную, остаются без advice. Порядок аспектов задаётся явно через `@Order` и проверяется по логу.
