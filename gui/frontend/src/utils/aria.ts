export const ARIA_ROLES = {
  MAIN: 'main',
  NAVIGATION: 'navigation',
  BUTTON: 'button',
  DIALOG: 'dialog',
  ALERTDIALOG: 'alertdialog',
  MENU: 'menu',
  MENUBAR: 'menubar',
  MENUITEM: 'menuitem',
  TAB: 'tab',
  TABLIST: 'tablist',
  TABPANEL: 'tabpanel',
  FORM: 'form',
  TEXTBOX: 'textbox',
  COMBOBOX: 'combobox',
  LISTBOX: 'listbox',
  OPTION: 'option',
  CHECKBOX: 'checkbox',
  RADIO: 'radio',
  SWITCH: 'switch',
  SLIDER: 'slider',
  SPINBUTTON: 'spinbutton',
  PROGRESSBAR: 'progressbar',
  STATUS: 'status',
  ALERT: 'alert',
  LOG: 'log',
  MARQUEE: 'marquee',
  TIMER: 'timer',
  TOOLTIP: 'tooltip',
} as const

export const ARIA_STATES = {
  HIDDEN: 'aria-hidden',
  DISABLED: 'aria-disabled',
  READONLY: 'aria-readonly',
  REQUIRED: 'aria-required',
  SELECTED: 'aria-selected',
  CHECKED: 'aria-checked',
  PRESSED: 'aria-pressed',
  EXPANDED: 'aria-expanded',
  BUSY: 'aria-busy',
  INVALID: 'aria-invalid',
  CURRENT: 'aria-current',
  HASPOPUP: 'aria-haspopup',
} as const

export const ARIA_PROPERTIES = {
  LABEL: 'aria-label',
  LABELLEDBY: 'aria-labelledby',
  DESCRIBEDBY: 'aria-describedby',
  LIVE: 'aria-live',
  ATOMIC: 'aria-atomic',
  RELEVANT: 'aria-relevant',
  POSINSET: 'aria-posinset',
  SETSIZE: 'aria-setsize',
  LEVEL: 'aria-level',
  VALUEMIN: 'aria-valuemin',
  VALUEMAX: 'aria-valuemax',
  VALUENOW: 'aria-valuenow',
  VALUETEXT: 'aria-valuetext',
  CONTROLS: 'aria-controls',
  OWNS: 'aria-owns',
  FLOWTO: 'aria-flowto',
  DESCRIBES: 'aria-describes',
  KEYSHORTCUTS: 'aria-keyshortcuts',
  ROLEDDESCRIPTION: 'aria-roledescription',
  AUTOCOMPLETE: 'aria-autocomplete',
  MULTILINE: 'aria-multiline',
  MULTISELECTABLE: 'aria-multiselectable',
  ORIENTATION: 'aria-orientation',
  PLACEHOLDER: 'aria-placeholder',
  SORT: 'aria-sort',
} as const

export const ARIA_LIVE_REGIONS = {
  POLITE: 'polite',
  ASSERTIVE: 'assertive',
  OFF: 'off',
} as const

export function createAriaProps(props: Record<string, string>): Record<string, string> {
  return props
}

export function getAriaLabel(elementType: string, context?: string): string {
  const labels: Record<string, string> = {
    'start-button': '启动代理服务',
    'stop-button': '停止代理服务',
    'config-button': '管理配置',
    'logs-button': '查看日志',
    'settings-button': '系统设置',
    'create-config': '新建配置',
    'import-config': '导入配置',
    'edit-config': '编辑配置',
    'delete-config': '删除配置',
    'export-config': '导出配置',
    'save-button': '保存配置',
    'cancel-button': '取消',
    'test-connection': '测试连接',
    'generate-key': '生成密钥',
    'install-cert': '安装 CA 证书',
    'uninstall-cert': '卸载 CA 证书',
    'clear-logs': '清空日志',
    'search-input': '搜索日志',
    'filter-select': '按级别过滤日志',
    'auto-scroll-toggle': '自动滚动开关',
    'close-dialog': '关闭对话框',
    'skip-onboarding': '跳过向导',
    'next-step': '下一步',
    'prev-step': '上一步',
    'complete-onboarding': '完成设置并开始使用',
  }

  if (context) {
    return `${labels[elementType] || elementType}: ${context}`
  }
  
  return labels[elementType] || elementType
}

export function announceToScreenReader(message: string, priority: 'polite' | 'assertive' = 'polite'): void {
  const announcement = document.createElement('div')
  announcement.setAttribute('role', 'status')
  announcement.setAttribute('aria-live', priority)
  announcement.setAttribute('aria-atomic', 'true')
  announcement.className = 'sr-only'
  announcement.textContent = message
  
  document.body.appendChild(announcement)
  
  setTimeout(() => {
    if (announcement.parentNode) {
      announcement.parentNode.removeChild(announcement)
    }
  }, 1000)
}

export function setupKeyboardNavigation(container: HTMLElement, options?: {
  onEscape?: () => void
  onEnter?: () => void
  onArrowUp?: () => void
  onArrowDown?: () => void
  onTab?: (shiftKey: boolean) => void
}): void {
  if (!container) return

  container.addEventListener('keydown', (event: KeyboardEvent) => {
    switch (event.key) {
      case 'Escape':
        event.preventDefault()
        options?.onEscape?.()
        break
      case 'Enter':
        if ((event.target as HTMLElement).tagName === 'BUTTON' || 
            (event.target as HTMLElement).getAttribute('role') === 'button') {
          options?.onEnter?.()
        }
        break
      case 'ArrowUp':
        event.preventDefault()
        options?.onArrowUp?.()
        break
      case 'ArrowDown':
        event.preventDefault()
        options?.onArrowDown?.()
        break
      case 'Tab':
        options?.onTab?.(event.shiftKey)
        break
    }
  })
}

export function setFocusTrap(container: HTMLElement): { activate: () => void; deactivate: () => void } {
  let previouslyFocused: HTMLElement | null = null
  let focusableElements: HTMLElement[] = []
  let firstFocusable: HTMLElement | null = null
  let lastFocusable: HTMLElement | null = null

  function getFocusableElements(): HTMLElement[] {
    if (!container) return []
    
    return Array.from(
      container.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )
    ).filter(el => el.offsetParent !== null || el.hasAttribute('tabindex'))
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key !== 'Tab') return

    if (!firstFocusable || !lastFocusable) return

    if (event.shiftKey) {
      if (document.activeElement === firstFocusable) {
        event.preventDefault()
        lastFocusable?.focus()
      }
    } else {
      if (document.activeElement === lastFocusable) {
        event.preventDefault()
        firstFocusable?.focus()
      }
    }
  }

  function activate() {
    previouslyFocused = document.activeElement as HTMLElement
    focusableElements = getFocusableElements()
    
    if (focusableElements.length > 0) {
      firstFocusable = focusableElements[0]
      lastFocusable = focusableElements[focusableElements.length - 1]
      firstFocusable.focus()
    }

    container.addEventListener('keydown', handleKeyDown)
  }

  function deactivate() {
    container.removeEventListener('keydown', handleKeyDown)
    previouslyFocused?.focus()
    previouslyFocused = null
    focusableElements = []
    firstFocusable = null
    lastFocusable = null
  }

  return { activate, deactivate }
}

export function createSkipLink(targetId: string): HTMLAnchorElement {
  const link = document.createElement('a')
  link.href = `#${targetId}`
  link.textContent = '跳转到主要内容'
  link.className = 'skip-link'
  link.setAttribute('role', 'link')
  link.style.cssText = `
    position: absolute;
    top: -40px;
    left: 0;
    background: #3b82f6;
    color: white;
    padding: 8px 16px;
    text-decoration: none;
    border-radius: 0 0 4px 0;
    z-index: 10000;
    transition: top 0.2s;
  `

  link.addEventListener('focus', () => {
    link.style.top = '0'
  })

  link.addEventListener('blur', () => {
    link.style.top = '-40px'
  })

  return link
}
