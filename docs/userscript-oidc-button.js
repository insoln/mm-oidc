// ==UserScript==
// @name         Mattermost OIDC Login Button
// @namespace    http://tampermonkey.net/
// @version      1.0.0
// @description  Adds "Sign in with OIDC" button to Mattermost login page
// @author       Your Organization
// @match        http://*/login
// @match        http://*/login?*
// @match        https://*/login
// @match        https://*/login?*
// @grant        none
// @run-at       document-end
// ==/UserScript==

(function() {
    'use strict';

    // ========== CONFIGURATION ==========
    // Change this to your Mattermost URL
    const MATTERMOST_URL = window.location.origin;
    const OIDC_LOGIN_PATH = '/plugins/com.mm.oidc/login';
    
    // Button styling options
    const BUTTON_TEXT = '👤 Sign in with OIDC';
    const BUTTON_COLOR = '#0058cc';
    const BUTTON_HOVER_COLOR = '#004bb3';
    const DIVIDER_TEXT = 'OR';
    
    // ========== MAIN CODE ==========
    
    function addOIDCButton() {
        // Check if we're actually on the login page
        if (!window.location.pathname.includes('/login')) {
            return;
        }
        
        // Find the login form
        const loginForm = document.querySelector('form');
        if (!loginForm) {
            console.log('[OIDC Button] Login form not found, retrying...');
            return false;
        }
        
        // Check if button already exists
        if (document.querySelector('.oidc-login-button')) {
            console.log('[OIDC Button] Button already exists');
            return true;
        }
        
        // Find the login button to insert after it
        const loginButton = loginForm.querySelector('button[type="submit"]');
        if (!loginButton) {
            console.log('[OIDC Button] Submit button not found');
            return false;
        }
        
        // Create container
        const container = document.createElement('div');
        container.className = 'oidc-button-container';
        container.style.cssText = 'width: 100%; margin-top: 16px;';
        
        // Create divider
        const divider = document.createElement('div');
        divider.className = 'oidc-divider';
        divider.style.cssText = `
            display: flex;
            align-items: center;
            text-align: center;
            margin: 24px 0 16px;
        `;
        divider.innerHTML = `
            <div style="flex: 1; border-bottom: 1px solid rgba(0, 0, 0, 0.16);"></div>
            <span style="padding: 0 16px; font-size: 13px; font-weight: 600; color: rgba(0, 0, 0, 0.56); text-transform: uppercase; letter-spacing: 0.02em;">${DIVIDER_TEXT}</span>
            <div style="flex: 1; border-bottom: 1px solid rgba(0, 0, 0, 0.16);"></div>
        `;
        
        // Create OIDC button
        const button = document.createElement('a');
        button.href = MATTERMOST_URL + OIDC_LOGIN_PATH;
        button.className = 'oidc-login-button';
        button.style.cssText = `
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            padding: 12px 20px;
            font-size: 16px;
            font-weight: 600;
            line-height: 1.5;
            color: white;
            background: ${BUTTON_COLOR};
            border: 1px solid rgba(0, 0, 0, 0.16);
            border-radius: 4px;
            cursor: pointer;
            transition: all 0.15s ease;
            gap: 12px;
            text-decoration: none;
            box-sizing: border-box;
        `;
        button.innerHTML = `
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" style="flex-shrink: 0;">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z" fill="currentColor"/>
            </svg>
            <span style="flex: 1; text-align: center;">${BUTTON_TEXT}</span>
        `;
        
        // Add hover effects
        button.addEventListener('mouseover', function() {
            this.style.background = BUTTON_HOVER_COLOR;
            this.style.borderColor = 'rgba(0, 0, 0, 0.24)';
            this.style.boxShadow = '0 2px 8px rgba(0, 0, 0, 0.08)';
            this.style.transform = 'translateY(-1px)';
        });
        
        button.addEventListener('mouseout', function() {
            this.style.background = BUTTON_COLOR;
            this.style.borderColor = 'rgba(0, 0, 0, 0.16)';
            this.style.boxShadow = 'none';
            this.style.transform = 'translateY(0)';
        });
        
        button.addEventListener('mousedown', function() {
            this.style.transform = 'translateY(0)';
            this.style.boxShadow = '0 1px 4px rgba(0, 0, 0, 0.08)';
        });
        
        // Create hint text
        const hint = document.createElement('p');
        hint.style.cssText = `
            margin-top: 8px;
            font-size: 13px;
            line-height: 1.5;
            color: rgba(0, 0, 0, 0.64);
            text-align: center;
        `;
        hint.textContent = 'Authenticate using your organization\'s identity provider';
        
        // Assemble components
        container.appendChild(divider);
        container.appendChild(button);
        container.appendChild(hint);
        
        // Insert after the login button
        loginButton.parentElement.insertBefore(container, loginButton.nextSibling);
        
        console.log('[OIDC Button] Successfully added OIDC login button');
        return true;
    }
    
    // Try to add button immediately
    if (!addOIDCButton()) {
        // If failed, wait for DOM to be ready and try again
        console.log('[OIDC Button] Waiting for page to load...');
        
        let attempts = 0;
        const maxAttempts = 20; // 10 seconds total
        
        const interval = setInterval(() => {
            attempts++;
            
            if (addOIDCButton()) {
                clearInterval(interval);
            } else if (attempts >= maxAttempts) {
                console.error('[OIDC Button] Failed to add button after', maxAttempts, 'attempts');
                clearInterval(interval);
            }
        }, 500);
    }
    
    // Watch for SPA navigation (Mattermost uses React Router)
    let lastUrl = location.href;
    new MutationObserver(() => {
        const url = location.href;
        if (url !== lastUrl) {
            lastUrl = url;
            // URL changed, try to add button again
            setTimeout(addOIDCButton, 100);
        }
    }).observe(document, {subtree: true, childList: true});
    
    console.log('[OIDC Button] UserScript loaded');
})();
