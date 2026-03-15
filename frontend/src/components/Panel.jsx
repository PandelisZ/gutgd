import { Card, Text } from '@fluentui/react-components'

export default function Panel({ title, description, actions, children, className = '' }) {
  return (
    <Card className={`gutgd-card ${className}`.trim()}>
      {(title || description || actions) ? (
        <div className="gutgd-cardHeader">
          <div>
            {title ? <Text as="h3" size={500} weight="semibold">{title}</Text> : null}
            {description ? <p>{description}</p> : null}
          </div>
          {actions}
        </div>
      ) : null}
      {children}
    </Card>
  )
}
