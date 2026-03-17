import { useEffect, useState } from 'react'

import PageHeader from '../components/PageHeader'
import AgentSettingsPanel from '../components/AgentSettingsPanel'
import { api } from '../lib/api'

export default function AgentSettingsView() {
  const [apiKey, setAPIKey] = useState('')
  const [model, setModel] = useState('gpt-5.4')
  const [models, setModels] = useState([])
  const [reasoningEffort, setReasoningEffort] = useState('medium')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [status, setStatus] = useState('')
  const [loadingSettings, setLoadingSettings] = useState(true)
  const [savingSettings, setSavingSettings] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    async function load() {
      try {
        const settings = await api.getAgentSettings()
        setAPIKey(settings.api_key || '')
        setModel(settings.model || 'gpt-5.4')
        setReasoningEffort(settings.reasoning_effort || 'medium')
        setSystemPrompt(settings.system_prompt || '')
        const options = await api.listAgentModels()
        setModels(options || [])
      } catch (nextError) {
        setError(nextError.message || String(nextError))
      } finally {
        setLoadingSettings(false)
      }
    }

    load()
  }, [])

  async function saveSettings() {
    setSavingSettings(true)
    setError('')
    setStatus('')
    try {
      const saved = await api.saveAgentSettings({
        api_key: apiKey,
        model,
        reasoning_effort: reasoningEffort,
        system_prompt: systemPrompt
      })
      setAPIKey(saved.api_key || '')
      setModel(saved.model || 'gpt-5.4')
      setReasoningEffort(saved.reasoning_effort || 'medium')
      setSystemPrompt(saved.system_prompt || '')
      setStatus('Settings saved.')
    } catch (nextError) {
      setError(nextError.message || String(nextError))
    } finally {
      setSavingSettings(false)
    }
  }

  return (
    <>
      <PageHeader
        title="Agent settings"
        description="Manage the desktop agent configuration separately from the live chat workspace."
      />

      <div className="gutgd-list">
        <AgentSettingsPanel
          apiKey={apiKey}
          model={model}
          models={models}
          reasoningEffort={reasoningEffort}
          systemPrompt={systemPrompt}
          status={status}
          error={error}
          loadingSettings={loadingSettings}
          savingSettings={savingSettings}
          onAPIKeyChange={setAPIKey}
          onModelChange={setModel}
          onReasoningEffortChange={setReasoningEffort}
          onSystemPromptChange={setSystemPrompt}
          onSave={saveSettings}
        />
      </div>
    </>
  )
}
