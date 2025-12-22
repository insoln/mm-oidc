# Исследование: Автоматический редирект на OIDC через прокси-контейнер

## Обзор

Данный документ содержит результаты исследования возможности настройки автоматического редиректа пользователей на страницу авторизации OIDC при обращении к контейнеризированному Mattermost через прокси-контейнер (NGINX, Traefik). Решение позволяет перенаправлять неавторизованных пользователей на OIDC без участия штатного плагина Mattermost.

## Архитектура решения

### Общая схема

```
Браузер пользователя
       ↓
Прокси-контейнер (NGINX/Traefik)
       ↓
   [проверка куки]
       ↓
   Есть MMAUTHTOKEN? → Да → Проксировать на Mattermost
       ↓ Нет
   Редирект на /plugins/com.mm.oidc/login
       ↓
   Mattermost OIDC Plugin
       ↓
   OIDC Provider (Keycloak)
       ↓
   Возврат на callback
       ↓
   Установка MMAUTHTOKEN cookie
       ↓
   Доступ к Mattermost
```

### Компоненты инфраструктуры

1. **Прокси-контейнер** (NGINX или Traefik):
   - Точка входа для всего HTTP-трафика
   - Проверяет наличие авторизационной куки
   - Выполняет условный редирект
   - Проксирует запросы к Mattermost

2. **Mattermost с OIDC Plugin**:
   - Обрабатывает OIDC-авторизацию
   - Устанавливает session cookie
   - Обслуживает основной функционал

3. **OIDC Provider** (Keycloak):
   - Identity Provider для аутентификации
   - Управление пользователями и ролями

## Определение наличия авторизационной куки

### Cookie Mattermost

Mattermost использует cookie с именем `MMAUTHTOKEN` для хранения токена сессии:

- **Имя**: `MMAUTHTOKEN`
- **Тип**: HTTP-only, Secure (при HTTPS)
- **Срок действия**: настраивается в конфигурации Mattermost
- **Путь**: `/`
- **Домен**: домен Mattermost сервера

### Логика определения в прокси

Прокси проверяет наличие cookie следующим образом:

```nginx
# NGINX
if ($cookie_MMAUTHTOKEN = "") {
    # Cookie отсутствует - редирект на OIDC
}
```

```yaml
# Traefik
middlewares:
  auth-check:
    plugin:
      cookie-check:
        cookieName: MMAUTHTOKEN
```

## Схемы интеграции

### Вариант 1: NGINX как reverse proxy

**Преимущества:**
- Простая конфигурация
- Широко распространен и хорошо документирован
- Высокая производительность
- Гибкие правила обработки запросов

**Недостатки:**
- Требует ручной настройки
- Менее динамичен по сравнению с Traefik
- Необходимость перезагрузки при изменении конфигурации

**Архитектура:**
```
Internet → NGINX (80/443) → Mattermost (8065)
                ↓
           Keycloak (8080)
```

### Вариант 2: Traefik как reverse proxy

**Преимущества:**
- Автоматическое обнаружение сервисов (с Docker labels)
- Динамическая конфигурация
- Встроенная поддержка Let's Encrypt
- Dashboard для мониторинга

**Недостатки:**
- Более сложная начальная настройка
- Требует middleware для проверки cookie
- Потенциально выше overhead

**Архитектура:**
```
Internet → Traefik (80/443) → Mattermost (8065)
                ↓
           Keycloak (8080)
```

## Порядок перенаправления на OIDC

### Последовательность действий

1. **Пользователь обращается к Mattermost** (`https://mattermost.example.com/`)

2. **Прокси проверяет наличие `MMAUTHTOKEN`**:
   - Если cookie присутствует и валиден → проксирование на Mattermost
   - Если cookie отсутствует → переход к шагу 3

3. **Редирект на OIDC login endpoint**:
   ```
   HTTP 302 Found
   Location: https://mattermost.example.com/plugins/com.mm.oidc/login?redirect_to=/
   ```

4. **OIDC Plugin запускает Authorization Code + PKCE flow**:
   - Генерирует state, nonce, code_verifier
   - Редиректит на Keycloak authorization endpoint

5. **Пользователь проходит аутентификацию в Keycloak**

6. **Keycloak возвращает код авторизации** на callback URL:
   ```
   https://mattermost.example.com/plugins/com.mm.oidc/callback?code=...&state=...
   ```

7. **OIDC Plugin обменивает код на токены**:
   - Проверяет state/nonce
   - Получает ID token и access token
   - Создает или обновляет пользователя в Mattermost
   - Устанавливает `MMAUTHTOKEN` cookie

8. **Редирект на исходный URL** (или главную страницу)

9. **Последующие запросы проходят через прокси** с валидной cookie

### Обработка исключений

Определенные URL должны быть **исключены** из логики редиректа:

- `/api/*` - API endpoints (могут использовать токены)
- `/plugins/com.mm.oidc/*` - endpoints самого плагина
- `/static/*` - статические ресурсы
- `/health` - health check endpoints
- WebSocket соединения

## Потенциальные сложности

### 1. Redirect Loops (циклические редиректы)

**Проблема**: Неправильная настройка может привести к бесконечным редиректам.

**Решение**:
- Исключить callback URL из проверки cookie
- Добавить проверку query параметров (например, `?oidc_redirect=true`)
- Использовать разные пути для разных типов запросов

### 2. API и WebSocket запросы

**Проблема**: API клиенты и WebSocket соединения не должны редиректиться.

**Решение**:
- Проверять заголовок `Accept` (API обычно использует `application/json`)
- Исключать WebSocket upgrade запросы (`Upgrade: websocket`)
- Создать whitelist путей для API

### 3. Статические ресурсы

**Проблема**: Редирект статических файлов нарушает загрузку страницы.

**Решение**:
- Исключить `/static/*` из проверки cookie
- Проверять Content-Type или расширение файла

### 4. CORS и Cross-Origin запросы

**Проблема**: Редиректы могут нарушать CORS политики.

**Решение**:
- Настроить CORS заголовки в прокси
- Пропускать preflight OPTIONS запросы без проверки cookie

### 5. Session Expiry и Token Refresh

**Проблема**: Cookie может устареть, но прокси все равно пропустит запрос.

**Решение**:
- Mattermost сам вернет 401 и прокси может обработать этот ответ
- Или прокси может проверять валидность cookie через внутренний API

### 6. Load Balancing и Session Affinity

**Проблема**: При multiple Mattermost инстансах cookie может быть невалидна на другом сервере.

**Решение**:
- Настроить sticky sessions в прокси
- Использовать shared session storage в Mattermost

## Рекомендации по безопасности

### 1. HTTPS Only

**Важно**: Всегда используйте HTTPS в production:
```nginx
server {
    listen 80;
    return 301 https://$server_name$request_uri;
}
```

### 2. Secure Cookie Flags

Убедитесь, что Mattermost устанавливает правильные флаги:
- `Secure` - только через HTTPS
- `HttpOnly` - недоступен из JavaScript
- `SameSite=Lax` или `SameSite=Strict` - защита от CSRF

### 3. Rate Limiting

Защита от brute-force и DoS:
```nginx
limit_req_zone $binary_remote_addr zone=login_limit:10m rate=5r/m;
limit_req zone=login_limit burst=10;
```

### 4. Headers Security

Добавьте security headers:
```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
```

### 5. Логирование и Аудит

Логируйте все попытки доступа без cookie:
```nginx
access_log /var/log/nginx/mattermost_access.log combined;
error_log /var/log/nginx/mattermost_error.log warn;
```

### 6. IP Whitelisting для Admin Console

Ограничьте доступ к `/admin` определенными IP:
```nginx
location /admin {
    allow 10.0.0.0/8;
    deny all;
}
```

### 7. Защита от Credential Stuffing

- Используйте CAPTCHA на странице логина Keycloak
- Включите account lockout policy в Keycloak
- Мониторьте подозрительную активность

## POC Конфигурации

### POC 1: NGINX Reverse Proxy

См. `deploy/mattermost-proxy/nginx.conf` и `deploy/docker-compose.dev.yml` для полной конфигурации.

**Основные моменты**:

```nginx
map $cookie_MMAUTHTOKEN $auth_redirect {
    default 0;
    "" 1;
}

server {
    listen 80;
    server_name mattermost.example.com;

    location / {
        # Проверяем исключения
        if ($request_uri ~* "^/(api|plugins/com.mm.oidc|static|health)") {
            set $auth_redirect 0;
        }

        # Проверяем WebSocket
        if ($http_upgrade = "websocket") {
            set $auth_redirect 0;
        }

        # Редиректим на OIDC если нет cookie
        if ($auth_redirect = 1) {
            return 302 /plugins/com.mm.oidc/login?redirect_to=$request_uri;
        }

        proxy_pass http://mattermost:8065;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket support
    location /api/v4/websocket {
        proxy_pass http://mattermost:8065;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### POC 2: Traefik Reverse Proxy

Ниже приведен референс-конфиг для Traefik (для самостоятельной адаптации).

**Основные моменты**:

```yaml
# docker-compose.yml
services:
  traefik:
    image: traefik:v3.0
    command:
      - --providers.docker=true
      - --entrypoints.web.address=:80
    ports:
      - "80:80"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  mattermost:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.mattermost.rule=Host(`mattermost.example.com`)"
      - "traefik.http.routers.mattermost.middlewares=oidc-redirect"
      - "traefik.http.middlewares.oidc-redirect.redirectregex.regex=^.*$$"
      - "traefik.http.middlewares.oidc-redirect.redirectregex.replacement=/plugins/com.mm.oidc/login"
```

**Примечание**: Для полноценной проверки cookie в Traefik потребуется custom middleware plugin.

## Инструкции по развертыванию

### Вариант 1: Встроенный NGINX (dev-стенд)

1. **Соберите и запустите стек**:
    ```bash
    scripts/dev-up.sh
    ```

2. **(Опционально) Запустите curl smoke-тесты**:
    ```bash
    scripts/test-proxy.sh
    ```

3. **(Опционально) Добавьте Playwright регрессию**:
    ```bash
    scripts/test-proxy-all.sh
    ```

4. **Откройте браузер** и перейдите на `http://mattermost-proxy.127.0.0.1.nip.io:8787`

5. **Проверьте редирект**:
    - Без cookie должен быть редирект на `/plugins/com.mm.oidc/login`
    - После успешной авторизации доступ к Mattermost

### Вариант 2: Traefik (пример для самостоятельной адаптации)

Реализация для Traefik не входит в репозиторий, но можно использовать приведённый выше snippet:

1. Создайте собственный `docker-compose.yml`, добавив сервис `traefik` и применив middleware `redirectregex`.
2. Пробросьте порт `80` (и при необходимости `8080` для dashboard).
3. Смонтируйте Docker socket, чтобы Traefik мог обнаружить сервисы.
4. Настройте правила `traefik.http.routers` для Mattermost и добавьте middleware, который редиректит на `/plugins/com.mm.oidc/login` при отсутствии cookie.
5. Протестируйте сценарии аналогично NGINX-варианту.

## Тестирование

### Сценарий 1: Первое посещение без cookie

```bash
# Запрос без cookie
curl -v http://mattermost-proxy.127.0.0.1.nip.io:8787/ 2>&1 | grep -E "(Location|HTTP)"

# Ожидается:
# HTTP/1.1 302 Found
# Location: /plugins/com.mm.oidc/login?redirect_to=/
```

### Сценарий 2: Запрос с валидной cookie

```bash
# Получите валидную MMAUTHTOKEN через браузер после логина
curl -v -H "Cookie: MMAUTHTOKEN=your_token_here" http://mattermost-proxy.127.0.0.1.nip.io:8787/ 2>&1 | grep HTTP

# Ожидается:
# HTTP/1.1 200 OK
```

### Сценарий 3: API запросы не должны редиректиться

```bash
curl -v http://mattermost-proxy.127.0.0.1.nip.io:8787/api/v4/users/me 2>&1 | grep HTTP

# Ожидается:
# HTTP/1.1 401 Unauthorized (но не 302 редирект)
```

### Сценарий 4: WebSocket соединения

```bash
curl -v -H "Upgrade: websocket" -H "Connection: upgrade" \
    http://mattermost-proxy.127.0.0.1.nip.io:8787/api/v4/websocket 2>&1 | grep HTTP

# Ожидается:
# HTTP/1.1 101 Switching Protocols (или проксирование на backend)
```

## Мониторинг и Отладка

### Логи NGINX

```bash
# Access logs
docker compose logs nginx | grep "GET /"

# Error logs
docker compose logs nginx | grep "error"
```

### Логи Traefik

```bash
# Traefik logs
docker compose logs traefik

# Access logs через dashboard
curl http://localhost:8080/api/rawdata
```

### Метрики

Настройте экспорт метрик для мониторинга:

**NGINX**:
```nginx
location /nginx_status {
    stub_status;
    allow 127.0.0.1;
    deny all;
}
```

**Traefik**:
```yaml
metrics:
  prometheus:
    entryPoint: metrics
```

## Производственные рекомендации

### 1. Используйте HTTPS

- Получите сертификаты от Let's Encrypt
- Настройте автоматическое обновление
- Используйте сильные cipher suites

### 2. High Availability

- Разверните несколько инстансов прокси
- Используйте load balancer (AWS ALB, GCP LB)
- Настройте health checks

### 3. Backup и Disaster Recovery

- Храните конфигурации в Git
- Регулярно делайте backup БД Mattermost и Keycloak
- Тестируйте процедуры восстановления

### 4. Масштабируемость

- Используйте кеширование статических ресурсов
- Настройте connection pooling
- Оптимизируйте worker processes

### 5. Compliance и Audit

- Включите подробное логирование
- Интегрируйте с SIEM системой
- Регулярно проводите security audits

## Альтернативные подходы

### 1. Использование Ingress Controllers (Kubernetes)

При развертывании в Kubernetes используйте ingress controllers:

- **NGINX Ingress Controller**
- **Traefik Ingress**
- **HAProxy Ingress**

Конфигурация через Ingress annotations или middleware CRDs.

### 2. API Gateway

Использование полноценного API Gateway:

- **Kong**
- **Tyk**
- **AWS API Gateway**

Предоставляют расширенные возможности аутентификации и авторизации.

### 3. Service Mesh

Для микросервисной архитектуры:

- **Istio**
- **Linkerd**
- **Consul Connect**

Предоставляют mTLS, advanced routing, и observability.

## Выводы

### Рекомендуемый подход

**Для большинства случаев рекомендуется NGINX**:
- Простота настройки
- Надежность и производительность
- Широкая поддержка сообщества
- Хорошо документирован

**Traefik подходит если**:
- Используете Docker/Kubernetes
- Нужна динамическая конфигурация
- Требуется автоматическое обнаружение сервисов

### Ключевые моменты

1. ✅ **Технически возможно** реализовать редирект через прокси
2. ✅ **Прокси-подход работает** для веб-интерфейса
3. ⚠️ **Требует осторожности** с API и WebSocket
4. ⚠️ **Необходимо тщательное тестирование** всех use cases
5. ✅ **Безопасно при правильной настройке** HTTPS, headers, rate limiting

### Ограничения

- Не подходит для API-only клиентов без браузера
- Требует дополнительной инфраструктуры
- Усложняет troubleshooting
- Может конфликтовать с mobile apps

### Применимость

**Подходит для**:
- Корпоративных развертываний с веб-only доступом
- Single sign-on сценариев
- Миграции с других систем аутентификации

**Не подходит для**:
- API-intensive приложений
- Mobile-first развертываний
- Когда требуется гибкость в методах аутентификации

## Следующие шаги

1. Протестировать POC в вашей среде
2. Адаптировать конфигурации под ваши требования
3. Провести нагрузочное тестирование
4. Подготовить runbook для операционной команды
5. Настроить мониторинг и алерты

## Ссылки и ресурсы

- [Mattermost OIDC Plugin Documentation](../README.md)
- [NGINX Reverse Proxy Guide](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/)
- [Traefik Documentation](https://doc.traefik.io/traefik/)
- [OpenID Connect Specification](https://openid.net/specs/openid-connect-core-1_0.html)
- [OAuth 2.0 Security Best Practices](https://tools.ietf.org/html/draft-ietf-oauth-security-topics)
