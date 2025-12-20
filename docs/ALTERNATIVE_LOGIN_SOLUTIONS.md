# Альтернативные способы добавления кнопки OIDC на страницу логина

Поскольку Mattermost Plugin API не поддерживает добавление компонентов на страницу логина, существуют альтернативные подходы **без использования плагинов**.

## 🎯 Рекомендуемые решения

### 1. ✅ Custom Branding (Встроенная функция)

**Описание:** Использовать встроенную функциональность Mattermost для добавления брендирования на страницу логина.

**Как настроить:**

1. Перейдите в **System Console → Site Configuration → Customization**
2. Включите **Enable Custom Branding**
3. Настройте:
   - **Site Name**: Название вашей организации
   - **Custom Brand Image**: Логотип (200-500px, <2MB)
   - **Custom Brand Text**: Инструкция для пользователей (до 500 символов, поддерживает Markdown)

**Пример Custom Brand Text:**

```markdown
## Вход через OIDC

Для входа используйте ваш корпоративный аккаунт:

👉 [**Войти через OIDC**](http://your-mattermost.com/plugins/com.mm.oidc/login)

Или используйте локальный аккаунт ниже.
```

**Преимущества:**
- ✅ Не требует изменения кода
- ✅ Безопасно для обновлений
- ✅ Поддерживает Markdown и ссылки
- ✅ Официально поддерживается

**Недостатки:**
- ⚠️ Ограниченные возможности стилизации
- ⚠️ Текст отображается над формой логина

**Визуальный результат:**
```
┌──────────────────────────────┐
│     [Ваш логотип]            │
│                              │
│  ## Вход через OIDC          │
│  Для входа используйте...    │
│  👉 Войти через OIDC (ссылка)│
│                              │
│  ─────────────────────────   │
│                              │
│  Email or Username           │
│  [________________]          │
│  Password                    │
│  [________________]          │
│  [ Sign in ]                 │
└──────────────────────────────┘
```

---

### 2. ✅ Nginx/Traefik Redirect + Custom Branding (Гибридный подход)

**Описание:** Комбинация ingress redirect для основного потока + custom branding для информирования пользователей.

**Преимущества:**
- ✅ Автоматический редирект для обычных пользователей
- ✅ Инструкции для администраторов (если нужен локальный логин)
- ✅ Лучший UX

**Custom Brand Text:**

```markdown
## Автоматический вход через OIDC

Вы будете автоматически перенаправлены на страницу входа через корпоративный аккаунт.

**Для администраторов:** Используйте форму ниже для локального входа.
```

---

### 3. 🔧 Browser Extension / UserScript (Для пользователей)

**Описание:** Использовать Tampermonkey/Greasemonkey для добавления кнопки на стороне клиента.

**UserScript пример:**

```javascript
// ==UserScript==
// @name         Mattermost OIDC Login Button
// @namespace    http://your-domain.com/
// @version      1.0
// @description  Add OIDC login button to Mattermost
// @match        http://your-mattermost.com/login*
// @match        https://your-mattermost.com/login*
// @grant        none
// ==/UserScript==

(function() {
    'use strict';
    
    // Ждем загрузки страницы
    window.addEventListener('load', function() {
        // Находим форму логина
        const loginForm = document.querySelector('form');
        if (!loginForm) return;
        
        // Создаем divider
        const divider = document.createElement('div');
        divider.style.cssText = 'display: flex; align-items: center; margin: 24px 0; text-align: center;';
        divider.innerHTML = '<div style="flex: 1; border-bottom: 1px solid #ccc;"></div>' +
                           '<span style="padding: 0 16px; font-size: 13px; font-weight: 600; color: #666;">OR</span>' +
                           '<div style="flex: 1; border-bottom: 1px solid #ccc;"></div>';
        
        // Создаем кнопку
        const button = document.createElement('a');
        button.href = '/plugins/com.mm.oidc/login';
        button.style.cssText = 'display: flex; align-items: center; justify-content: center; width: 100%; ' +
                              'padding: 12px 20px; font-size: 16px; font-weight: 600; color: white; ' +
                              'background: #0058cc; border: 1px solid #0058cc; border-radius: 4px; ' +
                              'text-decoration: none; margin-top: 16px; transition: all 0.15s ease;';
        button.innerHTML = '👤 Sign in with OIDC';
        
        button.addEventListener('mouseover', function() {
            this.style.background = '#004bb3';
        });
        button.addEventListener('mouseout', function() {
            this.style.background = '#0058cc';
        });
        
        // Добавляем на страницу
        loginForm.appendChild(divider);
        loginForm.appendChild(button);
    });
})();
```

**Установка:**
1. Установите Tampermonkey (Chrome) или Greasemonkey (Firefox)
2. Создайте новый скрипт
3. Вставьте код выше
4. Измените URL на ваш Mattermost
5. Сохраните

**Преимущества:**
- ✅ Работает без изменения сервера
- ✅ Каждый пользователь может установить
- ✅ Легко обновлять

**Недостатки:**
- ⚠️ Каждый пользователь должен установить расширение
- ⚠️ Не работает на мобильных устройствах

---

### 4. 🔧 Reverse Proxy Injection (Продвинутый)

**Описание:** Использовать nginx sub_filter для инъекции HTML/JavaScript в страницу логина.

**Nginx конфигурация:**

```nginx
location = /login {
    proxy_pass http://mattermost:8065;
    
    # Включаем модификацию контента
    sub_filter_once off;
    sub_filter_types text/html;
    
    # Инъекция JavaScript перед </body>
    sub_filter '</body>' '
        <script>
        (function() {
            window.addEventListener("load", function() {
                const loginForm = document.querySelector("form");
                if (!loginForm) return;
                
                // Создаем кнопку
                const btn = document.createElement("a");
                btn.href = "/plugins/com.mm.oidc/login";
                btn.style.cssText = "display:flex;align-items:center;justify-content:center;width:100%;padding:12px 20px;font-size:16px;font-weight:600;color:white;background:#0058cc;border:1px solid #0058cc;border-radius:4px;text-decoration:none;margin-top:16px;";
                btn.innerHTML = "👤 Sign in with OIDC";
                
                // Добавляем divider
                const div = document.createElement("div");
                div.style.cssText = "display:flex;align-items:center;margin:24px 0;";
                div.innerHTML = "<div style=\"flex:1;border-bottom:1px solid #ccc;\"></div><span style=\"padding:0 16px;font-size:13px;font-weight:600;color:#666;\">OR</span><div style=\"flex:1;border-bottom:1px solid #ccc;\"></div>";
                
                loginForm.appendChild(div);
                loginForm.appendChild(btn);
            });
        })();
        </script>
    </body>';
    
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

**Преимущества:**
- ✅ Работает для всех пользователей автоматически
- ✅ Не требует изменения Mattermost
- ✅ Можно кастомизировать любой HTML/CSS/JS

**Недостатки:**
- ⚠️ Требует доступ к nginx
- ⚠️ Может сломаться при обновлении Mattermost
- ⚠️ Нужно тестировать с каждой версией

---

### 5. 🛠️ Fork Mattermost Webapp (Максимальный контроль)

**Описание:** Форк репозитория mattermost-webapp и модификация исходного кода.

**Шаги:**

1. **Fork репозитория:**
```bash
git clone https://github.com/mattermost/mattermost.git
cd mattermost/webapp
git checkout -b custom-login-button
```

2. **Найдите компонент логина:**
```bash
# Обычно находится в:
# webapp/channels/src/components/login/login.tsx
```

3. **Добавьте кнопку OIDC:**
```tsx
// В файле login.tsx после основной формы логина

<div className="oidc-divider">
    <span>OR</span>
</div>

<a 
    href="/plugins/com.mm.oidc/login" 
    className="btn btn-primary oidc-login-button"
>
    <i className="fa fa-sign-in"></i>
    Sign in with OIDC
</a>
```

4. **Добавьте CSS:**
```css
/* В соответствующем CSS файле */
.oidc-divider {
    display: flex;
    align-items: center;
    margin: 24px 0;
}

.oidc-divider::before,
.oidc-divider::after {
    content: '';
    flex: 1;
    border-bottom: 1px solid #ccc;
}

.oidc-divider span {
    padding: 0 16px;
    font-size: 13px;
    font-weight: 600;
    color: #666;
}

.oidc-login-button {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    margin-top: 16px;
}
```

5. **Соберите webapp:**
```bash
make build
```

6. **Деплой:**
```bash
# Замените webapp на вашем сервере
cp -r dist/* /opt/mattermost/client/
```

**Преимущества:**
- ✅ Полный контроль над UI
- ✅ Нативная интеграция
- ✅ Лучшая производительность

**Недостатки:**
- ⚠️ Сложность обслуживания
- ⚠️ Нужно мержить изменения при обновлениях
- ⚠️ Требует знания React/TypeScript
- ⚠️ Нужна CI/CD pipeline для сборки

---

### 6. 🎨 CSS Injection via Custom Theme (Косметический)

**Описание:** Добавить CSS через кастомную тему для скрытия элементов и добавления инструкций.

**Ограничение:** Можно только стилизовать, не добавлять HTML элементы.

**Пример использования:**

1. Создайте custom theme JSON:
```json
{
    "sidebarBg": "#145dbf",
    "sidebarText": "#ffffff",
    ...
}
```

2. Добавьте в Custom Brand Text:
```markdown
## 🔐 Корпоративный вход

Нажмите на ссылку ниже для входа через OIDC:

### [**→ Войти через корпоративный аккаунт**](http://mattermost.com/plugins/com.mm.oidc/login)

---

*Или используйте локальный аккаунт:*
```

3. Используйте browser DevTools для получения CSS селекторов:
```css
/* Скрыть элементы, которые не нужны */
.login-form-title {
    display: none;
}

/* Выделить OIDC текст */
.custom-brand-text a {
    display: inline-block;
    padding: 12px 24px;
    background: #0058cc;
    color: white !important;
    border-radius: 4px;
    text-decoration: none;
    font-weight: 600;
    margin: 16px 0;
}
```

**Примечание:** CSS нельзя загрузить через UI, только через browser extension или nginx injection.

---

## 📊 Сравнение методов

| Метод | Сложность | Обслуживание | UX | Универсальность |
|-------|-----------|--------------|-----|-----------------|
| Custom Branding | ⭐ Легко | ⭐⭐⭐ Отлично | ⭐⭐ Хорошо | ⭐⭐⭐ Все платформы |
| Hybrid (Redirect + Branding) | ⭐⭐ Средне | ⭐⭐⭐ Отлично | ⭐⭐⭐ Отлично | ⭐⭐⭐ Все платформы |
| UserScript | ⭐ Легко | ⭐⭐ Хорошо | ⭐⭐⭐ Отлично | ⭐ Только desktop |
| Nginx Injection | ⭐⭐⭐ Сложно | ⭐⭐ Хорошо | ⭐⭐⭐ Отлично | ⭐⭐⭐ Все платформы |
| Fork Webapp | ⭐⭐⭐⭐ Очень сложно | ⭐ Плохо | ⭐⭐⭐ Отлично | ⭐⭐⭐ Все платформы |

## 🎯 Рекомендации по выбору

### Для быстрого старта
→ **Custom Branding** - просто добавьте ссылку в Custom Brand Text

### Для лучшего UX
→ **Hybrid Approach** - nginx redirect + custom branding для fallback

### Для максимального контроля  
→ **Nginx Injection** - если у вас есть доступ и опыт

### Для внутреннего использования
→ **UserScript** - быстро и просто для технических команд

### Для enterprise
→ **Fork Webapp** - только если у вас есть DevOps команда

## 📝 Примеры реализации

См. `docs/` директорию для детальных примеров:
- `LOGIN_INTEGRATION_GUIDE.md` - nginx/Traefik примеры
- `CUSTOM_BRANDING_EXAMPLES.md` - (этот файл) примеры custom branding
- `USERSCRIPT_TEMPLATE.js` - готовый userscript

## ⚠️ Важные замечания

1. **Безопасность:** Все методы безопасны, если ссылка ведет на ваш плагин
2. **Обновления:** Custom Branding - самый безопасный для обновлений
3. **Поддержка:** Официально поддерживается только Custom Branding
4. **Мобильные приложения:** Работают только server-side решения (nginx, fork)

## 🚀 Быстрый старт

**Самое простое решение (5 минут):**

1. Перейдите в System Console → Customization
2. Enable Custom Branding
3. В Custom Brand Text вставьте:
```markdown
[**→ Войти через OIDC**](http://your-mattermost.com/plugins/com.mm.oidc/login)
```
4. Сохраните
5. Готово!

Пользователи увидят ссылку и смогут кликнуть для OIDC логина.
