let runtimeReady = false
let checkCount = 0
const MAX_CHECK_COUNT = 50

function waitForRuntime(): Promise<void> {
  return new Promise((resolve) => {
    const check = () => {
      const rt = (window as any).runtime
      if (rt && rt.Invoke) {
        runtimeReady = true
        resolve()
        return
      }

      checkCount++
      if (checkCount >= MAX_CHECK_COUNT) {
        console.warn('Wails runtime 可能在开发模式中未正确加载')
        resolve()
        return
      }

      setTimeout(check, 100)
    }

    check()
  })
}

let initPromise: Promise<void> | null = null

export async function invoke(method: string, ...args: any[]): Promise<any> {
  if (!initPromise) {
    initPromise = waitForRuntime()
  }

  await initPromise

  const rt = (window as any).runtime
  if (!rt) {
    throw new Error('Wails runtime 未初始化 (runtime 对象不存在)')
  }
  if (!rt.Invoke) {
    throw new Error('Wails runtime 未初始化 (Invoke 方法不可用)')
  }

  try {
    return await rt.Invoke(`main.App.${method}`, ...args)
  } catch (err: any) {
    if (err?.message?.includes('runtime')) {
      throw new Error('Wails runtime 调用失败: ' + (err.message || '未知错误'))
    }
    throw err
  }
}

export function isRuntimeReady(): boolean {
  const rt = (window as any).runtime
  return !!(rt && rt.Invoke)
}