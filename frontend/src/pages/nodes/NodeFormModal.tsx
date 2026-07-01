import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Col, Form, Input, Modal, Row, Switch, message } from 'antd';

import { antdRule } from '@/utils/zodForm';
import { NodeFormSchema, type NodeFormValues } from '@/schemas/node';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { Msg } from '@/utils';

type Mode = 'add' | 'edit';

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  save: (payload: Partial<NodeRecord>) => Promise<Msg<unknown>>;
  onOpenChange: (open: boolean) => void;
  onAgentCreated?: (node: NodeRecord) => void;
}

function defaultValues(): NodeFormValues {
  return {
    name: '',
    remark: '',
    address: '',
    enable: true,
  };
}

export default function NodeFormModal({
  open,
  mode,
  node,
  save,
  onOpenChange,
  onAgentCreated,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<NodeFormValues>();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next: NodeFormValues = mode === 'edit' && node
      ? { ...base, ...(node as unknown as Partial<NodeFormValues>), id: node.id }
      : base;
    form.resetFields();
    form.setFieldsValue(next);
  }, [open, mode, node, form]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  async function onFinish(values: NodeFormValues) {
    const result = NodeFormSchema.safeParse(values);
    if (!result.success) {
      messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
      return;
    }
    setSubmitting(true);
    try {
      const payload: Partial<NodeRecord> = {
        id: result.data.id || 0,
        name: result.data.name.trim(),
        remark: result.data.remark?.trim() || '',
        address: result.data.address?.trim() || '',
        mode: 'agent',
        enable: result.data.enable,
      };
      const msg = await save(payload);
      if (msg?.success) {
        onOpenChange(false);
        if (msg.obj) {
          onAgentCreated?.(msg.obj as NodeRecord);
        }
      }
    } finally {
      setSubmitting(false);
    }
  }

  function close() {
    if (!submitting) onOpenChange(false);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        confirmLoading={submitting}
        okText={t('save')}
        cancelText={t('cancel')}
        width="520px"
        onOk={() => form.submit()}
        onCancel={close}
      >
        <Form form={form} layout="vertical" initialValues={defaultValues()} onFinish={onFinish}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            description={t('pages.nodes.agentModeHint')}
          />
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.name')}
                name="name"
                rules={[antdRule(NodeFormSchema.shape.name, t)]}
              >
                <Input placeholder={t('pages.nodes.namePlaceholder')} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.nodes.remark')} name="remark">
                <Input />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            label={t('pages.nodes.shareAddress')}
            name="address"
            extra={t('pages.nodes.shareAddressHint')}
          >
            <Input placeholder="example.com" />
          </Form.Item>
          <Form.Item label={t('pages.nodes.enable')} name="enable" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
