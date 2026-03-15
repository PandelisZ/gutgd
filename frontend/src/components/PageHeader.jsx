import { Text } from '@fluentui/react-components'

export default function PageHeader({ title, description, actions, kicker = 'Settings-style debugger' }) {
  return (
    <section className="gutgd-pageHeader">
      <div>
        <Text as="div" className="gutgd-pageKicker">{kicker}</Text>
        <h1>{title}</h1>
        <div className="gutgd-pageHeaderBody">{description}</div>
      </div>
      {actions ? <div className="gutgd-headerActions">{actions}</div> : null}
    </section>
  )
}
