import Script from "next/script";

import type { Tracking } from "../lib/types";

type Props = { tracking: Tracking };

// TrackingScripts renders 3rd-party tag snippets after page hydration. The
// site uses `strategy="afterInteractive"` so scripts don't block LCP.
// PDPA/GDPR consent gating is intentionally NOT implemented in Phase 1 — see
// plan.md §10.2; the consent banner ships in Phase 2 and will wrap these.
export default function TrackingScripts({ tracking }: Props) {
  return (
    <>
      {tracking.gtm_id && (
        <Script
          id="gtm-loader"
          strategy="afterInteractive"
          dangerouslySetInnerHTML={{
            __html: `
(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src='https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);})(window,document,'script','dataLayer','${tracking.gtm_id}');`,
          }}
        />
      )}

      {tracking.google_analytics_id && (
        <>
          <Script
            id="ga-loader"
            strategy="afterInteractive"
            src={`https://www.googletagmanager.com/gtag/js?id=${tracking.google_analytics_id}`}
          />
          <Script
            id="ga-init"
            strategy="afterInteractive"
            dangerouslySetInnerHTML={{
              __html: `window.dataLayer = window.dataLayer || []; function gtag(){dataLayer.push(arguments);} gtag('js', new Date()); gtag('config', '${tracking.google_analytics_id}');`,
            }}
          />
        </>
      )}

      {tracking.facebook_pixel_id && (
        <Script
          id="fbq-init"
          strategy="afterInteractive"
          dangerouslySetInnerHTML={{
            __html: `
!function(f,b,e,v,n,t,s){if(f.fbq)return;n=f.fbq=function(){n.callMethod?n.callMethod.apply(n,arguments):n.queue.push(arguments)};if(!f._fbq)f._fbq=n;n.push=n;n.loaded=!0;n.version='2.0';n.queue=[];t=b.createElement(e);t.async=!0;t.src=v;s=b.getElementsByTagName(e)[0];s.parentNode.insertBefore(t,s)}(window,document,'script','https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '${tracking.facebook_pixel_id}');
fbq('track', 'PageView');`,
          }}
        />
      )}

      {tracking.tiktok_pixel_id && (
        <Script
          id="ttq-init"
          strategy="afterInteractive"
          dangerouslySetInnerHTML={{
            __html: `
!function(w,d,t){w.TiktokAnalyticsObject=t;var ttq=w[t]=w[t]||[];ttq.methods=["page","track","identify","instances","debug","on","off","once","ready","alias","group","enableCookie","disableCookie"],ttq.setAndDefer=function(t,e){t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}};for(var i=0;i<ttq.methods.length;i++)ttq.setAndDefer(ttq,ttq.methods[i]);ttq.instance=function(t){for(var e=ttq._i[t]||[],n=0;n<ttq.methods.length;n++)ttq.setAndDefer(e,ttq.methods[n]);return e};ttq.load=function(e,n){var i="https://analytics.tiktok.com/i18n/pixel/events.js";ttq._i=ttq._i||{},ttq._i[e]=[],ttq._i[e]._u=i,ttq._t=ttq._t||{},ttq._t[e]=+new Date,ttq._o=ttq._o||{},ttq._o[e]=n||{};var o=document.createElement("script");o.type="text/javascript",o.async=!0,o.src=i+"?sdkid="+e+"&lib="+t;var a=document.getElementsByTagName("script")[0];a.parentNode.insertBefore(o,a)};
ttq.load('${tracking.tiktok_pixel_id}');
ttq.page();}(window, document, 'ttq');`,
          }}
        />
      )}

      {tracking.line_tag_id && (
        <Script
          id="line-tag-init"
          strategy="afterInteractive"
          dangerouslySetInnerHTML={{
            __html: `(function(g,d,o){g._ltq=g._ltq||[];g._lt=g._lt||function(){g._ltq.push(arguments)};var h=location.protocol==='https:'?'https://':'http://';var s=d.createElement('script');s.async=1;s.src=h+'d.line-scdn.net/n/line_tag/public/release/v1/lt.js';var t=d.getElementsByTagName('script')[0];t.parentNode.insertBefore(s,t);})(window,document);
_lt('init', { customerType: 'lap', tagId: '${tracking.line_tag_id}' });
_lt('send', 'pv', ['${tracking.line_tag_id}']);`,
          }}
        />
      )}
    </>
  );
}
