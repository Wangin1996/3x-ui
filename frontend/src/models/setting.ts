import { ObjectUtil } from '@/utils';

export class AllSetting {
  webListen = '';
  webDomain = '';
  webPort = 2053;
  webCertFile = '';
  webKeyFile = '';
  webBasePath = '/';
  sessionMaxAge = 360;
  trustedProxyCIDRs = '127.0.0.1/32,::1/128';
  panelOutbound = '';
  pageSize = 25;
  expireDiff = 0;
  trafficDiff = 0;
  remarkTemplate = '{{INBOUND}}-{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D';
  tgLang = 'en-US';
  twoFactorEnable = false;
  twoFactorToken = '';
  xrayTemplateConfig = '';
  subEnable = true;
  subJsonEnable = false;
  subTitle = '';
  subPath = '/sub/';
  subJsonPath = '/json/';
  subClashEnable = false;
  subClashPath = '/clash/';
  restartXrayOnClientDisable = true;
  subUpdates = 12;
  subEncrypt = true;
  subURI = '';
  subJsonURI = '';
  subClashURI = '';
  subClashEnableRouting = false;
  subClashRules = '';
  subClashTemplate = '';
  subJsonMux = '';
  subJsonRules = '';
  subJsonFinalMask = '';

  timeLocation = 'Local';

  smtpEnable = false;
  smtpHost = '';
  smtpPort = 587;
  smtpUsername = '';
  smtpPassword = '';
  smtpTo = '';
  smtpEncryptionType = 'starttls';
  smtpEnabledEvents = '';
  smtpCpu = 80;
  smtpMemory = 80;
  hasTwoFactorToken = false;
  hasApiToken = false;
  hasWarpSecret = false;
  hasNordSecret = false;
  hasSmtpPassword = false;

  constructor(data?: unknown) {
    if (data != null) {
      ObjectUtil.cloneProps(this, data);
    }
  }

  equals(other: AllSetting): boolean {
    return ObjectUtil.equals(this, other);
  }
}
