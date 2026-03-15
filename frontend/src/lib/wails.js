import { useEffect, useState } from 'react'

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function mockResult(method) {
  switch (method) {
    case 'GetDiagnostics':
      return {
        report: {
          ready: false,
          platform: 'browser-preview',
          backend: { name: 'mock', binding_state: 'no-wails-runtime' },
          reasons: ['The frontend is running in a plain browser preview. Open gutgd through Wails to call the live backend.']
        },
        feature_status: [],
        artifacts_path: '.artifacts',
        working_dir: '.',
        runtime: 'browser/mock'
      }
    case 'ListWindows':
      return [
        { handle: 101, title: 'Mock Notepad', region: { left: 80, top: 80, width: 900, height: 640 } },
        { handle: 102, title: 'Mock Calculator', region: { left: 200, top: 120, width: 480, height: 720 } }
      ]
    case 'GetActiveWindow':
    case 'FindWindowByTitle':
    case 'WaitForWindowByTitle':
      return { handle: 101, title: 'Mock Notepad', region: { left: 80, top: 80, width: 900, height: 640 } }
    case 'GetMousePosition':
      return { x: 640, y: 360 }
    case 'GetScreenSize':
      return { width: 1920, height: 1080 }
    case 'ColorAt':
      return {
        point: { x: 100, y: 100 },
        color: { r: 52, g: 120, b: 246, a: 255, hex: '#3478F6FF' },
        message: 'Mock color sample'
      }
    case 'ClipboardPaste':
      return { text: 'Mock clipboard text' }
    case 'ClipboardHasText':
      return { has_text: true }
    case 'GetAgentSettings':
      return { api_key: '', model: 'gpt-5.4', reasoning_effort: 'medium', system_prompt: '' }
    case 'ListAgentModels':
      return [{ id: 'gpt-5.4' }, { id: 'gpt-5-mini' }, { id: 'gpt-4.1' }]
    case 'SaveAgentSettings':
      return { api_key: '', model: 'gpt-5.4', reasoning_effort: 'medium', system_prompt: '' }
    case 'ChatWithAgent':
      return {
        message: {
          role: 'assistant',
          content: 'Browser preview mode cannot call the OpenAI-backed desktop agent. Launch gutgd through Wails to test tool calling.'
        },
        tool_events: [],
        response_id: 'preview-response',
        usage: { input_tokens: 0, output_tokens: 0, reasoning_tokens: 0, total_tokens: 0 }
      }
    case 'FindColor':
    case 'WaitForColor':
      return { x: 120, y: 88 }
    default:
      return { ok: true, message: `Mock response for ${method}` }
  }
}

export function hasDesktopRuntime() {
  if (typeof window === 'undefined') {
    return false
  }
  return Boolean(
    window.chrome?.webview
    || window.webkit?.messageHandlers
    || window.wails?.Call
  )
}

export function useBridgeMode() {
  const [mode, setMode] = useState('unavailable')

  useEffect(() => {
    if (hasDesktopRuntime()) {
      setMode('desktop')
      return
    }
    if (import.meta.env.DEV) {
      setMode('preview')
      return
    }
    setMode('unavailable')
  }, [])

  return mode
}

export function useBridgeReady() {
  const mode = useBridgeMode()
  return mode !== 'unavailable'
}

export async function callWithRuntime(method, task) {
  if (!hasDesktopRuntime()) {
    if (import.meta.env.DEV) {
      return clone(mockResult(method))
    }
    throw new Error('The Wails desktop runtime is unavailable. Start gutgd through Wails or run wails3 dev.')
  }
  return task()
}
