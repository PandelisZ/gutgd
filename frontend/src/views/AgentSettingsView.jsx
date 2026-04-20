import { useEffect, useState } from 'react'

import PageHeader from '../components/PageHeader'
import AgentSettingsPanel from '../components/AgentSettingsPanel'
import { api } from '../lib/api'

export default function AgentSettingsView() {
  const [apiKey, setAPIKey] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [model, setModel] = useState('gpt-5.4')
  const [models, setModels] = useState([])
  const [reasoningEffort, setReasoningEffort] = useState('medium')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [strictBackgroundOnly, setStrictBackgroundOnly] = useState(false)
  const [apiKeySource, setAPIKeySource] = useState('missing')
  const [status, setStatus] = useState('')
  const [loadingSettings, setLoadingSettings] = useState(true)
  const [savingSettings, setSavingSettings] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    async function load() {
      try {
        const [settings, settingsStatus] = await Promise.all([
          api.getAgentSettings(),
          api.getAgentSettingsStatus()
        ])
        setAPIKey(settings.api_key || '')
        setBaseURL(settings.base_url || '')
        setModel(settings.model || 'gpt-5.4')
        setReasoningEffort(settings.reasoning_effort || 'medium')
        setSystemPrompt(settings.system_prompt || '')
        setStrictBackgroundOnly(Boolean(settings.strict_background_only))
        setAPIKeySource(settingsStatus.api_key_source || 'missing')
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
        base_url: baseURL,
        model,
        reasoning_effort: reasoningEffort,
        system_prompt: systemPrompt,
        strict_background_only: strictBackgroundOnly
      })
      const settingsStatus = await api.getAgentSettingsStatus()
      setAPIKey(saved.api_key || '')
      setBaseURL(saved.base_url || '')
      setModel(saved.model || 'gpt-5.4')
      setReasoningEffort(saved.reasoning_effort || 'medium')
      setSystemPrompt(saved.system_prompt || '')
      setStrictBackgroundOnly(Boolean(saved.strict_background_only))
      setAPIKeySource(settingsStatus.api_key_source || 'missing')
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
          baseURL={baseURL}
          model={model}
          models={models}
          reasoningEffort={reasoningEffort}
          systemPrompt={systemPrompt}
          strictBackgroundOnly={strictBackgroundOnly}
          apiKeySource={apiKeySource}
          status={status}
          error={error}
          loadingSettings={loadingSettings}
          savingSettings={savingSettings}
          onAPIKeyChange={setAPIKey}
          onBaseURLChange={setBaseURL}
          onModelChange={setModel}
          onReasoningEffortChange={setReasoningEffort}
          onSystemPromptChange={setSystemPrompt}
          onStrictBackgroundOnlyChange={setStrictBackgroundOnly}
          onSave={saveSettings}
        />
      </div>
    </>
  )
}
