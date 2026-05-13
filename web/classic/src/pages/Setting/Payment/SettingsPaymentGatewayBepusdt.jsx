import React, { useEffect, useState, useRef } from 'react';
import {
  Banner,
  Button,
  Form,
  Row,
  Col,
  Spin,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsPaymentGatewayBepusdt(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    BepusdtEnabled: false,
    BepusdtHost: '',
    BepusdtToken: '',
    BepusdtUnitPrice: 1.0,
    BepusdtMinTopUp: 1,
    BepusdtNotifyUrl: '',
    BepusdtReturnUrl: '',
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        BepusdtEnabled: props.options.BepusdtEnabled === 'true' || props.options.BepusdtEnabled === true,
        BepusdtHost: props.options.BepusdtHost || '',
        BepusdtToken: props.options.BepusdtToken || '',
        BepusdtUnitPrice: parseFloat(props.options.BepusdtUnitPrice) || 1.0,
        BepusdtMinTopUp: parseInt(props.options.BepusdtMinTopUp) || 1,
        BepusdtNotifyUrl: props.options.BepusdtNotifyUrl || '',
        BepusdtReturnUrl: props.options.BepusdtReturnUrl || '',
      };
      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const submitSetting = async () => {
    setLoading(true);
    try {
      const options = [
        { key: 'BepusdtEnabled', value: inputs.BepusdtEnabled ? 'true' : 'false' },
        { key: 'BepusdtHost', value: inputs.BepusdtHost || '' },
        { key: 'BepusdtToken', value: inputs.BepusdtToken || '' },
        { key: 'BepusdtUnitPrice', value: String(inputs.BepusdtUnitPrice || 1.0) },
        { key: 'BepusdtMinTopUp', value: String(inputs.BepusdtMinTopUp || 1) },
        { key: 'BepusdtNotifyUrl', value: inputs.BepusdtNotifyUrl || '' },
        { key: 'BepusdtReturnUrl', value: inputs.BepusdtReturnUrl || '' },
      ];

      const results = await Promise.all(
        options.map((opt) => API.put('/api/option/', { key: opt.key, value: opt.value })),
      );

      const errors = results.filter((r) => !r.data.success);
      if (errors.length > 0) {
        errors.forEach((r) => showError(r.data.message));
      } else {
        showSuccess(t('更新成功'));
        props.refresh?.();
      }
    } catch {
      showError(t('更新失败'));
    }
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={(v) => setInputs(v)}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={t('BEpusdt USDT 支付设置')}>
          <Banner
            type='info'
            description={t('BEpusdt 是一个自建 USDT 收款系统，支持 TRC20/ERC20 等链。配置后用户可以用 USDT 充值。')}
          />

          <Row gutter={{ xs: 8, sm: 16, md: 24 }}>
            <Col xs={24} sm={24} md={8}>
              <Form.Switch
                field='BepusdtEnabled'
                label={t('启用 BEpusdt')}
                checkedText='｜'
                uncheckedText='〇'
              />
            </Col>
          </Row>

          <Row gutter={{ xs: 8, sm: 16, md: 24 }}>
            <Col xs={24} sm={24} md={12}>
              <Form.Input
                field='BepusdtHost'
                label={t('BEpusdt 服务地址')}
                placeholder='https://pay.example.com'
                extraText={t('部署 BEpusdt 的服务器地址，末尾不含斜杠')}
              />
            </Col>
            <Col xs={24} sm={24} md={12}>
              <Form.Input
                field='BepusdtToken'
                label={t('API Token')}
                placeholder={t('BEpusdt 后台的 API Token')}
                type='password'
              />
            </Col>
          </Row>

          <Row gutter={{ xs: 8, sm: 16, md: 24 }}>
            <Col xs={24} sm={24} md={8}>
              <Form.InputNumber
                field='BepusdtUnitPrice'
                label={t('单价 (USDT)')}
                placeholder='1.0'
                min={0}
                step={0.1}
                extraText={t('每个充值单位对应的 USDT 金额，默认 1.0')}
              />
            </Col>
            <Col xs={24} sm={24} md={8}>
              <Form.InputNumber
                field='BepusdtMinTopUp'
                label={t('最低充值数量')}
                placeholder='1'
                min={1}
                step={1}
              />
            </Col>
          </Row>

          <Row gutter={{ xs: 8, sm: 16, md: 24 }}>
            <Col xs={24} sm={24} md={12}>
              <Form.Input
                field='BepusdtNotifyUrl'
                label={t('回调通知地址（可选）')}
                placeholder={t('留空则自动使用 服务器地址/api/bepusdt/notify')}
              />
            </Col>
            <Col xs={24} sm={24} md={12}>
              <Form.Input
                field='BepusdtReturnUrl'
                label={t('支付返回地址（可选）')}
                placeholder={t('留空则自动跳转到充值页')}
              />
            </Col>
          </Row>

          <Button onClick={submitSetting} style={{ marginTop: 16 }}>
            {t('更新 BEpusdt 设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
