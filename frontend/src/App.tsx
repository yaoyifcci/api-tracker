import { Layout } from '@arco-design/web-react'
import RequestList from './pages/RequestList'
import TokenStatsFooter from './components/TokenStatsFooter'

const { Content } = Layout

export default function App() {
  return (
    <Layout style={{ minHeight: '100vh', background: '#f5f5f5' }}>
      <Content style={{ background: '#fff', minHeight: 'calc(100vh - 32px)' }}>
        <RequestList />
      </Content>
      <TokenStatsFooter />
    </Layout>
  )
}
