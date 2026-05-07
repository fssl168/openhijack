import { ref, computed, type Ref, type ComputedRef } from 'vue'

export interface AsyncState<T> {
  data: Ref<T | null>
  loading: Ref<boolean>
  error: Ref<string | null>
  isReady: ComputedRef<boolean>
  execute: (fn: () => Promise<T>) => Promise<T | null>
  reset: () => void
}

export function useAsyncState<T>(initialValue: T | null = null): AsyncState<T> {
  const data = ref<T | null>(initialValue) as Ref<T | null>
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isReady = computed(() => !loading.value && error.value === null && data.value !== null)

  async function execute(fn: () => Promise<T>): Promise<T | null> {
    loading.value = true
    error.value = null

    try {
      const result = await fn()
      data.value = result
      return result
    } catch (e: any) {
      error.value = e?.message || '操作失败'
      return null
    } finally {
      loading.value = false
    }
  }

  function reset() {
    data.value = initialValue
    loading.value = false
    error.value = null
  }

  return { data, loading, error, isReady, execute, reset }
}

export interface PaginationState {
  page: Ref<number>
  pageSize: Ref<number>
  total: Ref<number>
  totalPages: ComputedRef<number>
  hasNextPage: ComputedRef<boolean>
  hasPrevPage: ComputedRef<boolean>
  nextPage: () => void
  prevPage: () => void
  goToPage: (page: number) => void
  reset: () => void
}

export function usePagination(initialPageSize: number = 20): PaginationState {
  const page = ref(1)
  const pageSize = ref(initialPageSize)
  const total = ref(0)

  const totalPages = computed(() => Math.ceil(total.value / pageSize.value))
  const hasNextPage = computed(() => page.value < totalPages.value)
  const hasPrevPage = computed(() => page.value > 1)

  function nextPage() {
    if (hasNextPage.value) {
      page.value++
    }
  }

  function prevPage() {
    if (hasPrevPage.value) {
      page.value--
    }
  }

  function goToPage(newPage: number) {
    if (newPage >= 1 && newPage <= totalPages.value) {
      page.value = newPage
    }
  }

  function reset() {
    page.value = 1
    total.value = 0
  }

  return {
    page,
    pageSize,
    total,
    totalPages,
    hasNextPage,
    hasPrevPage,
    nextPage,
    prevPage,
    goToPage,
    reset,
  }
}

export interface SearchState {
  query: Ref<string>
  results: ComputedRef<any[]>
  setQuery: (query: string) => void
  clear: () => void
}

export function useSearch<T>(items: Ref<T[]>, searchFn: (item: T, query: string) => boolean): SearchState {
  const query = ref('')

  const results = computed(() => {
    if (!query.value.trim()) {
      return items.value
    }
    
    return items.value.filter(item => searchFn(item, query.value))
  })

  function setQuery(newQuery: string) {
    query.value = newQuery
  }

  function clear() {
    query.value = ''
  }

  return { query, results, setQuery, clear }
}
