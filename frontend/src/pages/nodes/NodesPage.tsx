import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Col, ConfigProvider, Layout, Modal, Result, Row, Spin, Statistic, message } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { useNodeMutations } from '@/api/queries/useNodeMutations';
import AppSidebar from '@/layouts/AppSidebar';
import NodeList from './NodeList';
import NodeFormModal from './NodeFormModal';
import AgentInstallModal from './AgentInstallModal';
import { setMessageInstance } from '@/utils/messageBus';

export default function NodesPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { nodes, loading, fetched, fetchError, refetch, totals } = useNodesQuery();
  const { create, update, remove, setEnable } = useNodeMutations();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formNode, setFormNode] = useState<NodeRecord | null>(null);
  const [installNode, setInstallNode] = useState<NodeRecord | null>(null);

  const onAdd = useCallback(() => {
    setFormMode('add');
    setFormNode(null);
    setFormOpen(true);
  }, []);

  const onEdit = useCallback((node: NodeRecord) => {
    setFormMode('edit');
    setFormNode({ ...node });
    setFormOpen(true);
  }, []);

  const onSave = useCallback(async (payload: Partial<NodeRecord>) => {
    if (formMode === 'edit' && formNode?.id) {
      return update(formNode.id, payload);
    }
    return create(payload);
  }, [formMode, formNode, update, create]);

  const onDelete = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.deleteConfirmTitle', { name: node.name }),
      content: t('pages.nodes.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.toasts.deleted'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onToggleEnable = useCallback(async (node: NodeRecord, next: boolean) => {
    await setEnable(node.id, next);
  }, [setEnable]);

  const pageClass = useMemo(() => {
    const classes = ['nodes-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" loading={loading} onClick={() => refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.nodes.totalNodes')}
                            value={String(totals.total)}
                            prefix={<CloudServerOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.nodes.onlineNodes')}
                            value={String(totals.online)}
                            prefix={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.nodes.offlineNodes')}
                            value={String(totals.offline)}
                            prefix={<CloseCircleOutlined style={{ color: 'var(--ant-color-error)' }} />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <NodeList
                      nodes={nodes}
                      loading={loading}
                      isMobile={isMobile}
                      onAdd={onAdd}
                      onEdit={onEdit}
                      onDelete={onDelete}
                      onToggleEnable={onToggleEnable}
                      onInstallCommand={setInstallNode}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <NodeFormModal
          open={formOpen}
          mode={formMode}
          node={formNode}
          save={onSave}
          onOpenChange={setFormOpen}
          onAgentCreated={setInstallNode}
        />

        <AgentInstallModal
          open={!!installNode}
          node={installNode}
          onClose={() => setInstallNode(null)}
        />
      </Layout>
    </ConfigProvider>
  );
}
