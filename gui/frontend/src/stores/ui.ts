import { defineStore } from 'pinia'
import type { ViewType } from '@/types'

export const useUIStore = defineStore('ui', {
  state: () => ({
    currentView: 'dashboard' as ViewType,
    sidebarCollapsed: false,
    configEditorOpen: false,
    editingConfig: null as any,
    notifications: [] as Array<{ id: number; message: string; type: 'success' | 'error' | 'info' | 'warn'; duration: number }>,
    nextNotificationId: 0,
  }),

  actions: {
    setView(view: ViewType) {
      this.currentView = view
    },

    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    },

    openConfigEditor(config: any = null) {
      this.editingConfig = config
      this.configEditorOpen = true
    },

    closeConfigEditor() {
      this.editingConfig = null
      this.configEditorOpen = false
    },

    showNotification(message: string, type: 'success' | 'error' | 'info' | 'warn' = 'info', duration = 3000) {
      const id = this.nextNotificationId++
      this.notifications.push({ id, message, type, duration })
      if (duration > 0) {
        setTimeout(() => {
          this.notifications = this.notifications.filter(n => n.id !== id)
        }, duration)
      }
    },

    dismissNotification(id: number) {
      this.notifications = this.notifications.filter(n => n.id !== id)
    },
  },
})
