# Лабораторная работа № 3. Конфигурация приложения (Go-версия)

**Студент:** Каримбаев Макжан, группа 2502IS · **Вариант:** 1 (`app.ui`) · **Ветка:** `lab03`

## 1. Цель работы

Вынести конфигурацию из кода: перевести проект на YAML, описать параметры типизированным объектом с валидацией, настроить профили для двух сред, экспериментально проверить порядок приоритета источников и научиться диагностировать конфигурацию через Actuator.

## 2. Что реализовано и как это соотносится со Spring Boot

| Spring Boot | Go (`pkg/env`, `pkg/ioc`) |
|---|---|
| `Environment` — упорядоченный список `PropertySource` | `env.Environment`, значение берётся из **первого** источника, где есть ключ |
| `application.yml`, `application-{profile}.yml` | тот же формат и те же имена файлов в `resources/` (парсер подмножества YAML с номерами строк для `Origin`) |
| `./config/application.yml` рядом с JAR | `./config/application.yml` рядом с бинарником |
| `--key=value`, `-Dkey=value`, `APP_MAIL_RETRYCOUNT` | те же три внешних источника |
| Relaxed binding | `retry-count` = `retryCount` = `APP_MAIL_RETRYCOUNT` = `--app.mail.retry-count` |
| `@ConfigurationProperties` + record | `env.Bind("app", &AppProperties{})` + Go-структура |
| `@Validated`, `@NotBlank`, `@Min`, `@Max`, `@Email`, `@DefaultValue`, `@Valid` | теги `validate:"notblank;min=1;max=10"`, `default:"3"`, `validate:"valid"` |
| `@ConfigurationPropertiesScan` | регистрация `BindAppProperties` как бина в `init()` пакета `config` |
| `@Profile("dev")`, `@Profile("!dev & !prod")` | `ioc.Profile("dev")`, `ioc.Profile("!dev & !prod")` |
| Автоконфигурация + `@ConditionalOnProperty`, `@ConditionalOnMissingBean` | `ioc.AutoConfiguration(...)` + `ioc.OnProperty`, `ioc.OnMissingBean[T]()`; обрабатываются **после** бинов приложения |
| Actuator: `/env`, `/configprops`, `/conditions`, `/health`; `--debug` | те же эндпоинты и тот же отчёт `CONDITIONS EVALUATION REPORT` |

Внешних зависимостей нет: YAML-парсер и валидатор написаны в проекте, `go.mod` по-прежнему пуст.

## 3. Задание 1. YAML и профильные файлы

`resources/application.properties` удалён. Созданы `application.yml` (общий), `application-dev.yml`, `application-prod.yml` — листинги в `resources/`. `spring.profiles.active` в общем файле **не задаётся**: профиль выбирается при запуске.

## 4. Задание 2. Типизированная конфигурация

Листинг — `internal/config/app_properties.go` (`AppProperties` с вложенными `Mail` и `UI`).

**Обязательный эксперимент:** `app.mail.retry-count: 99` →

```
***************************
APPLICATION FAILED TO START
***************************

Description:

Binding to target springlab/internal/config.AppProperties failed:

    Property: app.mail.retry-count
    Value: "99"
    Origin: class path resource [application.yml] - 15:18
    Reason: must be less than or equal to 10

Action:

Update your application's configuration
```

Сообщение называет ключ, полученное значение, файл со строкой и столбцом, и нарушенное ограничение `max=10`. Приложение не поднимается — ошибка поймана при старте, а не в работающей системе.

## 5. Задание 3. Профильные бины и `/api/lab3/config`

```
make run                                          # без профиля
make run ARGS="--spring.profiles.active=dev"
make run ARGS="--spring.profiles.active=prod"
./target/spring-lab-01 --spring.profiles.active=dev   # собранный бинарник
```

| | без профиля | dev | prod |
|---|---|---|---|
| `serverPort` | 8080 | **8081** | 8080 |
| `mailRetryCount` | 3 | **1** | **5** |
| `mailEnabled` | true | **false** | true |
| `activeProfiles` | `[]` | `["dev"]` | `["prod"]` |
| `banner` | no profile is active | DEVELOPMENT: the data is test data | PRODUCTION: handle with care |
| `uiTheme` / `uiItemsPerPage` (вариант) | LIGHT / 20 | DARK / 5 | LIGHT / 20 |

Полный ответ без профиля:

```json
{"activeProfiles":[],"banner":"no profile is active","group":"2502IS","mailEnabled":true,
 "mailFrom":"no-reply@iitu.kz","mailRetryCount":3,"mailTimeout":"5s","owner":"Karimbayev Makzhan",
 "serverPort":"8080","uiItemsPerPage":20,"uiTheme":"LIGHT"}
```

В профиле dev видно и действие `logging.level.kz.iitu: DEBUG` — в логе появляются строки `DEBUG Creating shared instance of singleton bean '...'`.

## 6. Задание 4. Приоритет источников

| Что задано | Ожидалось | Получено |
|---|---|---|
| только `application.yml` | 3 | **3** |
| yml + переменная окружения `APP_MAIL_RETRYCOUNT=7` | 7 | **7** |
| yml + переменная + `-Dapp.mail.retry-count=5` | 5 | **5** |
| yml + переменная + `--app.mail.retry-count=9` | 9 | **9** |
| профиль dev + `--app.mail.retry-count=9` | 9 | **9** |
| дополнительно: `./config/application.yml` с `retry-count: 4` | 4 | **4** |
| дополнительно: `APP_MAIL_RETRY_COUNT=7` (с подчёркиванием) | 3 | **3** |

**Почему победил аргумент командной строки.** Источники упорядочены от самого «внешнего» к самому «внутреннему»: аргументы → «системные свойства» `-D` → переменные окружения → `./config` → профильный файл внутри артефакта → общий файл внутри артефакта. Берётся первое найденное значение, поэтому аргумент перекрывает всё, включая профильный файл dev. Переменная `APP_MAIL_RETRY_COUNT` не действует: дефис в имени ключа просто удаляется, получается `RETRYCOUNT`, а не `RETRY_COUNT`.

## 7. Задание 5. Диагностика

`GET /actuator/env/app.mail.retry-count` при `APP_MAIL_RETRYCOUNT=7` и `--app.mail.retry-count=9`:

```json
{"property":{"source":"commandLineArgs","value":"9"},
 "propertySources":[
  {"name":"commandLineArgs","property":{"value":"9","origin":"\"--app.mail.retry-count=9\" from command line"}},
  {"name":"systemProperties"},
  {"name":"systemEnvironment","property":{"value":"7","origin":"System Environment Property \"APP_MAIL_RETRYCOUNT\""}},
  {"name":"Config resource 'class path resource [application.yml]' via location 'optional:classpath:/'",
   "property":{"value":"3","origin":"class path resource [application.yml] - 15:18"}}]}
```

Эндпоинт показывает и победителя, и всех проигравших — это самый быстрый ответ на вопрос «почему изменение в файле ни на что не влияет».

Отчёт `--debug` (профиль prod), фрагмент:

```
Positive matches:
   MailSenderAutoConfiguration#NewMailSender matched:
      - @ConditionalOnProperty(app.mail.enabled=true) found property 'app.mail.enabled' having value 'true'
   ClockAutoConfiguration#NewSystemClock matched:
      - @ConditionalOnMissingBean did not find any beans of type 'autoconfigure.Clock'
Negative matches:
   DataSourceAutoConfiguration#NewDataSource:
      Did not match:
         - @ConditionalOnProperty(spring.datasource.url=) did not find property 'spring.datasource.url'
```

- **Положительное совпадение** — `ClockAutoConfiguration`: приложение не объявило свой `Clock`, поэтому `@ConditionalOnMissingBean` разрешил создать бин по умолчанию. Если объявить собственный `Clock`, автоконфигурация уступит — это и есть правило «не отключайте автоконфигурацию, объявите свой бин».
- **Отрицательное совпадение** — `DataSourceAutoConfiguration`: в настройках нет `spring.datasource.url`, условие не выполнено, бин не создаётся.
- Интересный случай — `MailSenderAutoConfiguration`: в prod совпадение положительное, в dev — отрицательное (`found different value in property 'app.mail.enabled' (false)`), то есть профиль влияет и на автоконфигурацию.

Эндпоинты публикуются только из списка `management.endpoints.web.exposure.include`; значения ключей с `password`/`secret`/`token` маскируются как `******`.

## 8. Индивидуальное задание — вариант 1 (`app.ui`)

```go
type UI struct {
	Theme        string `default:"LIGHT" validate:"notblank;oneof=LIGHT DARK"`
	ItemsPerPage int    `default:"10" validate:"min=5;max=50"`
}
```

Значения заданы в `application.yml` (LIGHT / 20) и переопределены в `application-dev.yml` (DARK / 5), оба поля выводятся в `/api/lab3/config`. Проверка ограничения:

```
    Property: app.ui.items-per-page
    Value: "60"
    Origin: class path resource [application.yml] - 20:21
    Reason: must be less than or equal to 50
```

## 9. Типичные ошибки, воспроизведённые в работе

| Симптом | Причина |
|---|---|
| `while scanning for the next token found character '\t' ... line 15, column 1` | табуляция в YAML |
| значения из yml игнорируются | остался `application.properties` — он сильнее (тест `TestPropertiesBeatYAMLInSameLocation`) |
| `APP_MAIL_RETRY_COUNT` не действует | нужно `APP_MAIL_RETRYCOUNT` |

## 10. Ответы на ключевые вопросы

- **Почему профиль не задаётся в основном файле?** Тогда один и тот же артефакт в production запустится с профилем из файла (например, dev). Профиль — свойство среды запуска, а не сборки.
- **Когда активен dev, что читается?** Оба файла: общий всегда, профильный поверх него.
- **Зачем `@Valid` (`validate:"valid"`) на вложенном объекте?** Без него ограничения вложенной структуры не проверяются — в Go-версии это так же, как в Spring.
- **Как передать настройку в Docker без пересборки?** Переменной окружения: `docker run -e APP_MAIL_RETRYCOUNT=5 ...` или `-e SPRING_PROFILES_ACTIVE=prod` (проверено: `SPRING_PROFILES_ACTIVE=dev` активирует профиль).

## 11. Выводы

Один и тот же бинарник работает в трёх средах, а всё изменяемое задаётся снаружи. Порядок «чем внешнее источник, тем он сильнее» подтверждён экспериментом, а `/actuator/env` позволяет увидеть, откуда взялось каждое значение. Валидация при старте превращает ошибку конфигурации в отказ запуска с понятным сообщением. Трудности: в песочнице были недоступны внешние Go-модули, поэтому YAML-парсер и валидатор написаны вручную; по ходу найдена и исправлена ошибка с комментарием сразу после ключа.
