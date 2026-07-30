import Link from "next/link";

/**
 * The marketplace trail reuses the docs masthead crumb treatment (`.site-doc-masthead__*` in
 * `app/global.css`): accent product word, hairline separators, plain leaf. Two destinations on the
 * same site should not have two different breadcrumb languages.
 */
export interface MarketplaceCrumb {
  name: string;
  href: string;
}

export function MarketplaceCrumbs({
  trail = [],
  leaf,
}: {
  trail?: MarketplaceCrumb[];
  leaf: string;
}) {
  return (
    <nav aria-label="Breadcrumb" className="site-doc-masthead__crumbs">
      <Link href="/marketplace" className="site-doc-masthead__product">
        Marketplace
      </Link>
      {trail.map(crumb => (
        <span key={crumb.href} className="contents">
          <span aria-hidden className="site-doc-masthead__crumb-sep" />
          <Link href={crumb.href}>{crumb.name}</Link>
        </span>
      ))}
      <span aria-hidden className="site-doc-masthead__crumb-sep" />
      <span aria-current="page" className="site-doc-masthead__leaf">
        {leaf}
      </span>
    </nav>
  );
}
