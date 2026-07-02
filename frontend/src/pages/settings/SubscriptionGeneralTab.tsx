import { Input, InputNumber, Switch, Tabs } from 'antd';
import { IdcardOutlined, InfoCircleOutlined, NodeIndexOutlined, SettingOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { RemarkTemplateField } from '@/components/form';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import { sanitizePath, normalizePath } from './uriPath';

interface SubscriptionGeneralTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export default function SubscriptionGeneralTab({ allSetting, updateSetting }: SubscriptionGeneralTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();

  return (
    <Tabs defaultActiveKey="1" items={[
      {
        key: '1',
        label: catTabLabel(<SettingOutlined />, t('pages.settings.panelSettings'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subEnable')} description={t('pages.settings.subEnableDesc')}>
              <Switch checked={allSetting.subEnable} onChange={(v) => updateSetting({ subEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subJsonEnableTitle')} description={t('pages.settings.subJsonEnable')}>
              <Switch checked={allSetting.subJsonEnable} onChange={(v) => updateSetting({ subJsonEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subClashEnableTitle')}>
              <Switch checked={allSetting.subClashEnable} onChange={(v) => updateSetting({ subClashEnable: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subPath')} description={t('pages.settings.subPathDesc')}>
              <Input
                value={allSetting.subPath}
                placeholder="/sub/"
                onChange={(e) => updateSetting({ subPath: sanitizePath(e.target.value) })}
                onBlur={() => updateSetting({ subPath: normalizePath(allSetting.subPath) })}
              />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subURI')} description={t('pages.settings.subURIDesc')}>
              <Input value={allSetting.subURI} placeholder="(http|https)://domain[:port]/path/"
                onChange={(e) => updateSetting({ subURI: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '2',
        label: catTabLabel(<InfoCircleOutlined />, t('pages.settings.information'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subEncrypt')} description={t('pages.settings.subEncryptDesc')}>
              <Switch checked={allSetting.subEncrypt} onChange={(v) => updateSetting({ subEncrypt: v })} />
            </SettingListItem>
            <SettingListItem
              paddings="small"
              title={t('pages.settings.remarkTemplate')}
              description={t('pages.settings.remarkTemplateDesc')}
            >
              <RemarkTemplateField
                value={allSetting.remarkTemplate}
                onChange={(v) => updateSetting({ remarkTemplate: v })}
                maxLength={256}
              />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.subUpdates')} description={t('pages.settings.subUpdatesDesc')}>
              <InputNumber value={allSetting.subUpdates} min={1} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ subUpdates: Number(v) || 0 })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '3',
        label: catTabLabel(<IdcardOutlined />, t('pages.settings.profile'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subTitle')} description={t('pages.settings.subTitleDesc')}>
              <Input value={allSetting.subTitle} onChange={(e) => updateSetting({ subTitle: e.target.value })} />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '6',
        label: catTabLabel(<NodeIndexOutlined />, 'Clash / Mihomo', isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subClashEnableRouting')} description={t('pages.settings.subClashEnableRoutingDesc')}>
              <Switch checked={allSetting.subClashEnableRouting} onChange={(v) => updateSetting({ subClashEnableRouting: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.subClashRoutingRules')} description={t('pages.settings.subClashRoutingRulesDesc')}>
              <Input.TextArea
                value={allSetting.subClashRules}
                rows={8}
                placeholder={'GEOSITE,category-ir,DIRECT\nGEOIP,private,DIRECT'}
                onChange={(e) => updateSetting({ subClashRules: e.target.value })}
              />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
