import React, { useEffect, useState, useContext, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Skeleton, Tag, Tooltip } from '@douyinfe/semi-ui';
import { UserContext } from '../../context/User';
import { API, showError, showSuccess, renderQuota } from '../../helpers';
import { getCurrencyConfig } from '../../helpers/render';
import {
  formatSubscriptionDuration,
  formatSubscriptionResetPeriod,
} from '../../helpers/subscriptionFormat';
import SubscriptionPurchaseModal from '../../components/topup/modals/SubscriptionPurchaseModal';
import {
  Zap,
  Shield,
  Sparkles,
  Clock,
  Database,
  RefreshCw,
  Users,
  ArrowUpRight,
  Check,
  Crown,
} from 'lucide-react';

function getEpayMethods(payMethods = []) {
  return (payMethods || []).filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem',
  );
}

function submitEpayForm({ url, params }) {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';
  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) form.target = '_blank';
  Object.keys(params || {}).forEach((key) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = params[key];
    form.appendChild(input);
  });
  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

const SubscriptionPlans = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [userState] = useContext(UserContext);
  const isLoggedIn = !!userState?.user;

  const [plans, setPlans] = useState([]);
  const [loading, setLoading] = useState(true);
  const [payMethods, setPayMethods] = useState([]);
  const [enableOnlineTopUp, setEnableOnlineTopUp] = useState(false);
  const [enableStripeTopUp, setEnableStripeTopUp] = useState(false);
  const [enableCreemTopUp, setEnableCreemTopUp] = useState(false);
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [allSubscriptions, setAllSubscriptions] = useState([]);

  // Purchase modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState(null);
  const [paying, setPaying] = useState(false);
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('');

  const epayMethods = useMemo(() => getEpayMethods(payMethods), [payMethods]);

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map();
    (allSubscriptions || []).forEach((sub) => {
      const planId = sub?.subscription?.plan_id;
      if (!planId) return;
      map.set(planId, (map.get(planId) || 0) + 1);
    });
    return map;
  }, [allSubscriptions]);

  const openBuy = (p) => {
    if (!isLoggedIn) {
      navigate('/login');
      return;
    }
    setSelectedPlan(p);
    setSelectedEpayMethod(epayMethods?.[0]?.type || '');
    setModalOpen(true);
  };

  const closeBuy = () => {
    setModalOpen(false);
    setSelectedPlan(null);
    setPaying(false);
  };

  const payStripe = async () => {
    if (!selectedPlan?.plan?.stripe_price_id) {
      showError(t('该套餐未配置 Stripe'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/stripe/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.pay_link, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        showError(res.data?.data || res.data?.message || t('支付失败'));
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payCreem = async () => {
    if (!selectedPlan?.plan?.creem_product_id) {
      showError(t('该套餐未配置 Creem'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/creem/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.checkout_url, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        showError(res.data?.data || res.data?.message || t('支付失败'));
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payEpay = async () => {
    if (!selectedEpayMethod) {
      showError(t('请选择支付方式'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/epay/pay', {
        plan_id: selectedPlan.plan.id,
        payment_method: selectedEpayMethod,
      });
      if (res.data?.message === 'success') {
        submitEpayForm({ url: res.data.url, params: res.data.data });
        showSuccess(t('已发起支付'));
        closeBuy();
      } else {
        showError(res.data?.data || res.data?.message || t('支付失败'));
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const getPlans = async () => {
    setLoading(true);
    try {
      const endpoint = isLoggedIn
        ? '/api/subscription/plans'
        : '/api/subscription/public/plans';
      const res = await API.get(endpoint);
      if (res.data?.success) {
        setPlans(res.data.data || []);
      }
    } catch (e) {
      setPlans([]);
    } finally {
      setLoading(false);
    }
  };

  const getSubscriptionSelf = useCallback(async () => {
    if (!isLoggedIn) return;
    try {
      const res = await API.get('/api/subscription/self');
      if (res.data?.success) {
        setActiveSubscriptions(res.data.data?.subscriptions || []);
        setAllSubscriptions(res.data.data?.all_subscriptions || []);
      }
    } catch (e) {
      // ignore
    }
  }, [isLoggedIn]);

  const getTopupInfo = async () => {
    if (!isLoggedIn) return;
    try {
      const res = await API.get('/api/user/topup/info');
      const { data, success } = res.data;
      if (success) {
        let methods = data.pay_methods || [];
        if (typeof methods === 'string') methods = JSON.parse(methods);
        methods = (methods || []).filter((m) => m?.name && m?.type);
        setPayMethods(methods);
        setEnableOnlineTopUp(data.enable_online_topup || false);
        setEnableStripeTopUp(data.enable_stripe_topup || false);
        setEnableCreemTopUp(data.enable_creem_topup || false);
      }
    } catch (e) {
      // ignore
    }
  };

  useEffect(() => {
    getPlans();
    getSubscriptionSelf();
    getTopupInfo();
  }, [isLoggedIn]);

  const heroFeatures = [
    { icon: Zap, label: t('即时生效') },
    { icon: Shield, label: t('安全支付') },
    { icon: Sparkles, label: t('灵活升级') },
  ];

  const renderPlanCard = (p, index) => {
    const plan = p?.plan;
    const { symbol, rate } = getCurrencyConfig();
    const price = Number(plan?.price_amount || 0);
    const convertedPrice = price * rate;
    const displayPrice = convertedPrice.toFixed(
      Number.isInteger(convertedPrice) ? 0 : 2,
    );
    const totalAmount = Number(plan?.total_amount || 0);
    const isPopular = index === 0 && plans.length > 1;
    const limit = Number(plan?.max_purchase_per_user || 0);
    const count = planPurchaseCountMap.get(plan?.id) || 0;
    const reached = limit > 0 && count >= limit;

    const features = [
      {
        icon: Clock,
        label: `${t('有效期')} ${formatSubscriptionDuration(plan, t)}`,
      },
      totalAmount > 0
        ? { icon: Database, label: `${t('总额度')} ${renderQuota(totalAmount)}` }
        : { icon: Database, label: `${t('总额度')} ${t('不限')}` },
    ];

    const resetLabel = formatSubscriptionResetPeriod(plan, t);
    if (resetLabel !== t('不重置')) {
      features.push({ icon: RefreshCw, label: `${t('额度重置')} ${resetLabel}` });
    }
    if (limit > 0) {
      features.push({ icon: Users, label: `${t('限购')} ${limit} ${t('次')}` });
    }
    if (plan?.upgrade_group) {
      features.push({
        icon: ArrowUpRight,
        label: `${t('升级分组')} ${plan.upgrade_group}`,
      });
    }

    return (
      <div
        key={plan?.id}
        className='relative flex flex-col h-full cursor-pointer'
        style={{
          borderRadius: '20px',
          padding: isPopular ? '2px' : '0',
          background: isPopular
            ? 'linear-gradient(135deg, #667eea 0%, #764ba2 50%, #f093fb 100%)'
            : 'transparent',
        }}
      >
        <div
          className='relative flex flex-col h-full p-6'
          style={{
            borderRadius: isPopular ? '18px' : '20px',
            background: 'var(--semi-color-bg-0)',
            border: isPopular ? 'none' : '1px solid var(--semi-color-border)',
            transition: 'box-shadow 0.3s ease, transform 0.3s ease',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.boxShadow = isPopular
              ? '0 20px 60px rgba(102,126,234,0.3)'
              : '0 12px 40px rgba(0,0,0,0.1)';
            e.currentTarget.style.transform = 'translateY(-4px)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.boxShadow = 'none';
            e.currentTarget.style.transform = 'translateY(0)';
          }}
        >
          {/* Popular badge */}
          {isPopular && (
            <div
              className='absolute -top-3 left-1/2 -translate-x-1/2 flex items-center gap-1.5 px-4 py-1.5 rounded-full text-white text-xs font-semibold'
              style={{
                background:
                  'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                boxShadow: '0 4px 15px rgba(102,126,234,0.4)',
              }}
            >
              <Crown size={12} />
              {t('推荐')}
            </div>
          )}

          {/* Header */}
          <div className={isPopular ? 'mt-3' : ''}>
            <h3
              className='text-xl font-bold mb-1'
              style={{ color: 'var(--semi-color-text-0)' }}
            >
              {plan?.title || t('订阅套餐')}
            </h3>
            {plan?.subtitle && (
              <p
                className='text-sm mb-4'
                style={{ color: 'var(--semi-color-text-2)' }}
              >
                {plan.subtitle}
              </p>
            )}
          </div>

          {/* Price */}
          <div className='py-4'>
            <div className='flex items-baseline gap-1'>
              <span
                className='text-lg font-semibold'
                style={{
                  color: isPopular
                    ? '#764ba2'
                    : 'var(--semi-color-text-2)',
                }}
              >
                {symbol}
              </span>
              <span
                className='text-5xl font-extrabold tracking-tight'
                style={{
                  color: isPopular
                    ? '#764ba2'
                    : 'var(--semi-color-text-0)',
                }}
              >
                {displayPrice}
              </span>
              <span
                className='text-sm ml-1'
                style={{ color: 'var(--semi-color-text-2)' }}
              >
                / {formatSubscriptionDuration(plan, t)}
              </span>
            </div>
          </div>

          {/* Divider */}
          <div
            className='w-full h-px my-2'
            style={{ background: 'var(--semi-color-border)' }}
          />

          {/* Features */}
          <div className='flex-1 py-4 space-y-3'>
            {features.map(({ icon: Icon, label }) => (
              <div key={label} className='flex items-center gap-3'>
                <div
                  className='flex-shrink-0 w-5 h-5 rounded-full flex items-center justify-center'
                  style={{
                    background: isPopular
                      ? 'rgba(102,126,234,0.12)'
                      : 'rgba(var(--semi-green-5),0.12)',
                  }}
                >
                  <Check
                    size={12}
                    style={{
                      color: isPopular
                        ? '#667eea'
                        : 'rgba(var(--semi-green-5),1)',
                    }}
                    strokeWidth={3}
                  />
                </div>
                <span
                  className='text-sm'
                  style={{ color: 'var(--semi-color-text-1)' }}
                >
                  {label}
                </span>
              </div>
            ))}
          </div>

          {/* CTA Button */}
          <div className='mt-4'>
            {reached ? (
              <Tooltip
                content={`${t('已达到购买上限')} (${count}/${limit})`}
                position='top'
              >
                <button
                  disabled
                  className='w-full py-3 rounded-xl text-sm font-semibold opacity-50 cursor-not-allowed'
                  style={{
                    background: 'var(--semi-color-fill-0)',
                    color: 'var(--semi-color-text-2)',
                  }}
                >
                  {t('已达上限')}
                </button>
              </Tooltip>
            ) : (
              <button
                className='w-full py-3 rounded-xl text-sm font-semibold cursor-pointer'
                style={{
                  background: isPopular
                    ? 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)'
                    : 'var(--semi-color-fill-0)',
                  color: isPopular ? '#fff' : 'var(--semi-color-text-0)',
                  border: isPopular
                    ? 'none'
                    : '1px solid var(--semi-color-border)',
                  transition: 'all 0.2s ease',
                  boxShadow: isPopular
                    ? '0 4px 15px rgba(102,126,234,0.3)'
                    : 'none',
                }}
                onMouseEnter={(e) => {
                  if (isPopular) {
                    e.currentTarget.style.boxShadow =
                      '0 6px 20px rgba(102,126,234,0.45)';
                  } else {
                    e.currentTarget.style.background =
                      'var(--semi-color-primary-light-default)';
                    e.currentTarget.style.color = 'var(--semi-color-primary)';
                    e.currentTarget.style.borderColor =
                      'var(--semi-color-primary)';
                  }
                }}
                onMouseLeave={(e) => {
                  if (isPopular) {
                    e.currentTarget.style.boxShadow =
                      '0 4px 15px rgba(102,126,234,0.3)';
                  } else {
                    e.currentTarget.style.background =
                      'var(--semi-color-fill-0)';
                    e.currentTarget.style.color = 'var(--semi-color-text-0)';
                    e.currentTarget.style.borderColor =
                      'var(--semi-color-border)';
                  }
                }}
                onClick={() => openBuy(p)}
              >
                {isLoggedIn ? t('立即订阅') : t('登录后订阅')}
              </button>
            )}
          </div>
        </div>
      </div>
    );
  };

  const renderSkeletonCards = () => (
    <div className='grid grid-cols-1 md:grid-cols-3 gap-6'>
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className='p-6'
          style={{
            borderRadius: '20px',
            border: '1px solid var(--semi-color-border)',
            background: 'var(--semi-color-bg-0)',
          }}
        >
          <Skeleton.Title active style={{ width: '50%', height: 24, marginBottom: 8 }} />
          <Skeleton.Paragraph active rows={1} style={{ marginBottom: 16 }} />
          <Skeleton.Title active style={{ width: '40%', height: 48, marginBottom: 16 }} />
          <Skeleton.Paragraph active rows={3} />
          <Skeleton.Button active block style={{ marginTop: 24, height: 44, borderRadius: 12 }} />
        </div>
      ))}
    </div>
  );

  return (
    <div className='min-h-[80vh]'>
      {/* Hero Section */}
      <div
        className='relative overflow-hidden'
        style={{
          background:
            'linear-gradient(135deg, #667eea 0%, #764ba2 50%, #f093fb 100%)',
        }}
      >
        <div
          className='absolute top-0 left-0 w-72 h-72 rounded-full opacity-20 blur-3xl'
          style={{ background: '#f093fb' }}
        />
        <div
          className='absolute bottom-0 right-0 w-96 h-96 rounded-full opacity-15 blur-3xl'
          style={{ background: '#667eea' }}
        />

        <div className='relative max-w-4xl mx-auto px-4 py-16 text-center'>
          <h1 className='text-4xl md:text-5xl font-bold text-white mb-4 tracking-tight'>
            {t('选择适合你的套餐')}
          </h1>
          <p className='text-lg md:text-xl text-white/80 mb-8 max-w-2xl mx-auto'>
            {t('灵活的订阅方案，满足不同规模的使用需求')}
          </p>

          <div className='flex flex-wrap items-center justify-center gap-3'>
            {heroFeatures.map(({ icon: Icon, label }) => (
              <div
                key={label}
                className='flex items-center gap-2 px-4 py-2 rounded-full'
                style={{
                  background: 'rgba(255,255,255,0.15)',
                  backdropFilter: 'blur(8px)',
                  border: '1px solid rgba(255,255,255,0.2)',
                }}
              >
                <Icon size={16} className='text-white' />
                <span className='text-sm font-medium text-white'>{label}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Plans Section */}
      <div className='max-w-5xl mx-auto px-4 -mt-8 relative z-10'>
        {loading ? (
          renderSkeletonCards()
        ) : plans.length > 0 ? (
          <div className='grid grid-cols-1 md:grid-cols-3 gap-6'>
            {plans.map((p, i) => renderPlanCard(p, i))}
          </div>
        ) : (
          <div
            className='text-center py-16 rounded-2xl'
            style={{
              background: 'var(--semi-color-bg-0)',
              border: '1px solid var(--semi-color-border)',
            }}
          >
            <p style={{ color: 'var(--semi-color-text-2)' }}>
              {t('暂无可购买套餐')}
            </p>
          </div>
        )}
      </div>

      <div className='h-16' />

      {/* Purchase Modal */}
      <SubscriptionPurchaseModal
        t={t}
        visible={modalOpen}
        onCancel={closeBuy}
        selectedPlan={selectedPlan}
        paying={paying}
        selectedEpayMethod={selectedEpayMethod}
        setSelectedEpayMethod={setSelectedEpayMethod}
        epayMethods={epayMethods}
        enableOnlineTopUp={enableOnlineTopUp}
        enableStripeTopUp={enableStripeTopUp}
        enableCreemTopUp={enableCreemTopUp}
        purchaseLimitInfo={
          selectedPlan?.plan?.id
            ? {
                limit: Number(selectedPlan?.plan?.max_purchase_per_user || 0),
                count: planPurchaseCountMap.get(selectedPlan?.plan?.id) || 0,
              }
            : null
        }
        onPayStripe={payStripe}
        onPayCreem={payCreem}
        onPayEpay={payEpay}
      />
    </div>
  );
};

export default SubscriptionPlans;
