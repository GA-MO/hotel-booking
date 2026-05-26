import { SUPPORTED_LOCALES, type Locale } from "../i18n";

type Props = {
  current: Locale;
  pathname: string;
};

// LanguageSwitcher is intentionally simple — TH/EN only, anchor links that
// flip the `?lang=` param. Plain `<a>` (not next/link) because the pathname
// is dynamic and `typedRoutes` would otherwise reject the href; a full
// re-render is also desired so the new dictionary applies.
export default function LanguageSwitcher({ current, pathname }: Props) {
  return (
    <nav aria-label="Language" className="flex items-center gap-2 text-sm">
      {SUPPORTED_LOCALES.map((l) => {
        const isActive = l === current;
        return (
          <a
            key={l}
            href={`${pathname}?lang=${l}`}
            className={
              isActive
                ? "rounded px-2 py-1 font-semibold underline underline-offset-4"
                : "rounded px-2 py-1 text-neutral-600 hover:text-neutral-900"
            }
            aria-current={isActive ? "page" : undefined}
          >
            {l.toUpperCase()}
          </a>
        );
      })}
    </nav>
  );
}
