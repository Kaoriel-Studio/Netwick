# Netwick

[English](README.md) · **Русский**

Бесплатная программа с открытым кодом для Windows. Выясняет, почему Riot Client или игра не запускается,
не обновляется или не подключается, и исправляет это только способами из официальных статей Riot и Microsoft.

> **Netwick не связан с Riot Games.** Программа не трогает игру и античит, не просит пароль
> и работает только с настройками Windows.
>
> Netwick isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone
> officially involved in producing or managing Riot Games properties. Riot Games and all associated
> properties are trademarks or registered trademarks of Riot Games, Inc.

## Что делает

Нажмите **«Проверить компьютер»**, и примерно за 10 секунд Netwick:

- прочитает настройки Windows и файлы установки Riot (TPM, Secure Boot, брандмауэр, службы, свободное место, `hosts`, VPN и т. д.);
- проверит, отвечают ли публичные серверы Riot, — только когда вы нажали кнопку;
- простыми словами объяснит причину и предложит решение;
- применит исправление **только с вашего разрешения**, а любое изменение можно отменить одной кнопкой;
- запустит Riot Client так же, как это делает его собственный ярлык.

## Чего никогда не делает

- Не трогает игру, её файлы и память. Никаких оверлеев, внедрения кода и перехвата трафика.
- Не отключает, не останавливает и не обходит античит Vanguard.
- Не просит и не хранит логин и пароль от Riot.
- Не нажимает клавиши и не кликает за вас: никаких макросов и автопринятия матча.
- Не показывает чужую статистику и скрытую информацию, не использует API Riot.
- Ничего никуда не отправляет: ни телеметрии, ни рекламы. Не ставит VPN, прокси и «ускорители».

Подробнее: [правила Riot о сторонних приложениях](https://support.riotgames.com/en-us/valorant/account/third-party-applications).

## Установка

**Требования:** Windows 10 (сборка 19041 и новее) или Windows 11, x64, Microsoft Edge WebView2
(он уже есть в Windows 11 и в большинстве установок Windows 10).

1. Скачайте `netwick.exe` и `SHA256SUMS` со страницы [Releases](https://github.com/Kaoriel-Studio/Netwick/releases).
2. Запустите `netwick.exe`. Программа пока не подписана сертификатом, поэтому SmartScreen покажет окно
   **«Windows защитила ваш компьютер»** — нажмите **«Подробнее» → «Выполнить в любом случае»**.
3. Netwick запускается **без** прав администратора. Windows запросит разрешение (UAC) только тогда, когда вы выберете исправление, которому оно нужно.

### Как убедиться, что файл настоящий

- Хеш: в PowerShell выполните `Get-FileHash .\netwick.exe -Algorithm SHA256` и сравните с `SHA256SUMS`.
- Происхождение сборки (подтверждает, что exe собран GitHub Actions из этого репозитория):
  `gh attestation verify netwick.exe --repo Kaoriel-Studio/Netwick`
- В каждом релизе есть ссылка на проверку VirusTotal.

### Ложные срабатывания антивируса

Неподписанные программы на Go иногда ошибочно считают вредоносными. Netwick не открывает скрытых окон PowerShell,
не добавляет себя в автозагрузку и никогда не скачивает исполняемые файлы. Если антивирус ругается, сообщите
о ложном срабатывании его разработчику и создайте issue здесь, указав название обнаружения.

## Приватность и безопасность

Никакие данные о вас и вашем компьютере не покидают его. Подробности:

- [docs/PRIVACY.md](docs/PRIVACY.md) — с какими данными работает Netwick
- [docs/SECURITY.md](docs/SECURITY.md) — модель угроз и чек-лист безопасности
- [SECURITY.md](SECURITY.md) — как сообщить об уязвимости

## Для разработчиков

**Стек:** Go 1.27.3, [Wails v3](https://v3alpha.wails.io/) (3.0.0-beta.34), TypeScript без фреймворков + Vite. Без CGO на Windows.

### Подготовка

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
cd frontend && npm ci && cd ..
```

### Запуск на macOS или Linux (демо-режим)

Netwick рассчитан на Windows, но на других системах работает в **демо-режиме**: Windows имитируется,
на вашем компьютере ничего не меняется. Один раз соберите фронтенд (Go встраивает `frontend/dist`), затем запустите:

```bash
(cd frontend && npm run build)
go run . --demo=all-ok          # или переменная NETWICK_DEMO=<сценарий>
```

Сценарии: `all-ok`, `owner-real-run`, `no-internet`, `isp-cuts-some-ips`, `tpm-off-win11`, `secure-boot-off`,
`vgc-disabled`, `firewall-blocks`, `vpn-on`, `van-68`, `van-128`, `old-hosts-entries`.

В релизной сборке для Windows демо-режима нет.

### Основные команды

| Команда | Что делает |
| --- | --- |
| `wails3 dev` | Режим разработки с горячей перезагрузкой (вне Windows — демо-режим) |
| `go test ./...` | Все тесты Go, включая TLS-стенд |
| `cd frontend && npm run lint && npm run build && npm test` | Линт, сборка и проверки фронтенда |
| `wails3 build GOOS=windows VERSION=2.0.0` | Собирает `bin/netwick.exe` (x64, запуск с правами пользователя) |

`wails3 task test` запускает проверки Go и фронтенда вместе.

CI заново генерирует биндинги Wails и падает, если `frontend/bindings` отличается от закоммиченного. Поэтому
коммитьте обновлённые биндинги вместе с изменениями в Go, которые их вызвали.

### Дополнительно

- [docs/DECISIONS.md](docs/DECISIONS.md) — принятые решения
- [docs/TESTING-WINDOWS.md](docs/TESTING-WINDOWS.md) — ручной чек-лист проверки на настоящем Windows-ПК

## Лицензия

[MIT](LICENSE)
