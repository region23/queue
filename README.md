# Telegram-бот выполняющий функции автомата электронной очереди :)

Простой бот для записи в очередь с уведомлениями.

## Настройка

1. Скопируйте `.env.example` в `.env` и заполните переменные:

   ```bash
   cp .env.example .env
   ```

2. Установите ngrok (если не установлен):

   ```bash
   # macOS
   brew install ngrok
   
   # Linux
   curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
   echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
   sudo apt update && sudo apt install ngrok
   ```

## Использование

### Доступные команды Makefile

- `make help` - Показать справку по командам
- `make build` - Собрать бота
- `make run` - Запустить бота с переменными окружения
- `make ngrok` - Запустить ngrok туннель
- `make webhook` - Установить webhook URL в Telegram
- `make dev` - Запустить среду разработки (ngrok + бот)
- `make clean` - Очистить артефакты сборки
- `make env` - Показать переменные окружения

### Быстрый старт для разработки

1. Запустить ngrok в отдельном терминале:

   ```bash
   make ngrok
   ```

2. В другом терминале запустить бота:

   ```bash
   make run
   ```

3. Или запустить все одной командой:

   ```bash
   make dev
   ```

### Переменные окружения (.env)

- `TELEGRAM_TOKEN` - Токен бота от BotFather
- `WEBHOOK_URL` - URL для webhook (<https://frankly-wanted-polliwog.ngrok-free.app/webhook>)
- `WORK_START` - Начало рабочего дня (формат HH:MM)
- `WORK_END` - Конец рабочего дня (формат HH:MM)
- `SLOT_DURATION` - Длительность слота в минутах
- `SCHEDULE_DAYS` - Количество дней для планирования (при значении 1 сразу показываются слоты на сегодня)
- `DB_FILE` - Путь к файлу базы данных SQLite
- `PORT` - Порт для веб-сервера (по умолчанию 8080)

## Функционал

1. Пользователь отправляет `/start`
2. Бот просит поделиться номером телефона
3. В зависимости от настройки `SCHEDULE_DAYS`:
   - Если `SCHEDULE_DAYS=1`: сразу показывает доступные слоты на сегодня (только будущие времена)
   - Если `SCHEDULE_DAYS>1`: пользователь выбирает дату, затем временной слот
4. Пользователь выбирает свободный временной слот
5. Бот отправляет уведомление, когда подходит очередь

### Особенности при SCHEDULE_DAYS=1

- Пропускается этап выбора даты
- Показываются только слоты на сегодняшний день
- Автоматически фильтруются слоты, которые уже прошли (по текущему времени)

## Структура проекта

```text
github.com/region23/queue/
├── cmd/
│   └── server/
│       └── main.go           # Точка входа приложения
├── internal/
│   ├── bot/
│   │   ├── service/          # Сервисный слой бота
│   │   │   └── service.go
│   │   ├── handlers/         # Обработчики сообщений
│   │   │   ├── start.go      # Обработчик /start
│   │   │   ├── contact.go    # Обработчик контактов
│   │   │   ├── callback.go   # Обработчик callback'ов
│   │   │   └── default.go    # Обработчик по умолчанию
│   │   ├── keyboard/         # Клавиатуры
│   │   │   └── keyboards.go
│   │   └── dispatcher/       # Диспетчер обновлений
│   │       └── dispatcher.go
│   ├── config/               # Конфигурация
│   │   └── config.go
│   ├── storage/              # Хранилище данных
│   │   ├── interfaces.go     # Интерфейсы
│   │   ├── models/           # Модели данных
│   │   │   └── models.go
│   │   └── sqlite/           # SQLite реализация
│   │       └── sqlite.go
│   ├── scheduler/            # Планировщик уведомлений
│   │   ├── interfaces.go
│   │   └── memory/
│   │       └── scheduler.go
│   ├── server/               # HTTP сервер
│   │   ├── server.go
│   │   ├── middleware.go
│   │   └── ...
│   ├── middleware/           # Middleware компоненты
│   │   └── ratelimit.go
│   └── validation/           # Валидация
│       └── validation.go
├── pkg/
│   ├── logger/               # Логирование
│   │   └── logger.go
│   └── errors/               # Обработка ошибок
│       └── errors.go
├── tests/                    # Тесты
│   ├── testutils/
│   ├── unit/
│   └── integration/
├── docs/                     # Документация
├── Makefile                  # Команды для сборки и запуска
├── .env                      # Файл с переменными окружения
├── queue.db                  # База данных SQLite (создается автоматически)
└── main_old.go              # Старая монолитная версия (резервная копия)
```
