import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';

// ─── Feature list ────────────────────────────────────────────────────────────

type FeatureItem = {
  emoji: string;
  title: string;
  description: JSX.Element;
};

const FeatureList: FeatureItem[] = [
  {
    emoji: '🔗',
    title: 'Short Links',
    description: (
      <>
        Create memorable slugs like <code>go/slack</code> or <code>go/deploy</code>{' '}
        that redirect to any URL. Share them across your team instantly.
      </>
    ),
  },
  {
    emoji: '🌐',
    title: 'Browser Extension',
    description: (
      <>
        Type <code>go/jira</code> directly in Chrome, Firefox, or Safari. The
        extension intercepts bare-hostname navigation without a trailing slash or
        protocol prefix.
      </>
    ),
  },
  {
    emoji: '⌨️',
    title: 'Keyword Templates',
    description: (
      <>
        Parameterized links let <code>go/jira/&#123;ticket&#125;</code> expand to
        your full Jira URL with the ticket filled in — just like Google's internal
        go links.
      </>
    ),
  },
  {
    emoji: '🔑',
    title: 'REST API & PATs',
    description: (
      <>
        Full REST API secured with Personal Access Tokens. Script link management,
        integrate with CI/CD, or build your own tooling on top of joe&#x2011;links.
      </>
    ),
  },
  {
    emoji: '🔒',
    title: 'Visibility Controls',
    description: (
      <>
        Links can be public, private, or secure. Secure links are only accessible
        to specific users you invite — perfect for sensitive destinations.
      </>
    ),
  },
  {
    emoji: '👥',
    title: 'Co-ownership',
    description: (
      <>
        Multiple users can own and manage any link. Shared team shortcuts stay
        up&#x2011;to&#x2011;date without a single point of failure.
      </>
    ),
  },
];

// ─── Components ──────────────────────────────────────────────────────────────

function Feature({ emoji, title, description }: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className={styles.featureCard}>
        <div className={styles.featureIcon}>{emoji}</div>
        <Heading as="h3" className={styles.featureTitle}>
          {title}
        </Heading>
        <p className={styles.featureDesc}>{description}</p>
      </div>
    </div>
  );
}

function HomepageHeader() {
  return (
    <header className={styles.heroBanner}>
      <div className="container">
        <div className={styles.heroBadge}>Self-hosted go-links</div>
        <Heading as="h1" className={styles.heroTitle}>
          joe&#x2011;links
        </Heading>
        <p className={styles.heroTagline}>
          Short memorable URLs for your homelab or team.
          <br />
          Single binary. OIDC auth. No drama.
        </p>
        <div className={styles.buttons}>
          <Link
            className="button button--primary button--lg"
            to="/guides/getting-started"
          >
            Get Started →
          </Link>
          <Link
            className="button button--outline button--secondary button--lg"
            href="https://github.com/joestump/joe-links"
          >
            GitHub ↗
          </Link>
        </div>
      </div>
    </header>
  );
}

function HomepageFeatures(): JSX.Element {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function QuickStart(): JSX.Element {
  return (
    <section className={styles.quickStart}>
      <div className="container">
        <div className="row">
          <div className="col col--8 col--offset-2">
            <Heading as="h2" className="text--center margin-bottom--lg">
              Quick Start
            </Heading>
            <div className={styles.codeBlock}>
              <pre>
                <code>
                  {`# Run with Docker Compose
cp .env.example .env
# Edit .env with your OIDC provider details
docker compose up -d
# Visit http://localhost:8080

# Or download the binary
curl -L https://github.com/joestump/joe-links/releases/latest/\\
download/joe-links-linux-amd64 -o joe-links && chmod +x joe-links
JOE_DB_DRIVER=sqlite3 JOE_DB_DSN=./joe-links.db ./joe-links serve`}
                </code>
              </pre>
            </div>
            <p className="text--center margin-top--lg">
              <Link to="/guides/getting-started">
                View full setup guide →
              </Link>
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

function AIDisclosure(): JSX.Element {
  return (
    <section className={styles.aiDisclosure}>
      <div className="container">
        <p>
          🤖 joe-links was designed and written by{' '}
          <Link href="https://www.anthropic.com/claude">Claude</Link>{' '}
          (Anthropic's AI assistant) in collaboration with{' '}
          <Link href="https://github.com/joestump">Joe Stump</Link>.
          The full source code, architecture decisions, and specifications are{' '}
          <Link href="https://github.com/joestump/joe-links">open source</Link>.
        </p>
      </div>
    </section>
  );
}

// ─── Page ────────────────────────────────────────────────────────────────────

export default function Home(): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Self-hosted go-links"
      description="Self-hosted go-links service. Short memorable URLs for your homelab or team. Single binary, OIDC auth, browser extensions for Chrome, Firefox, and Safari."
    >
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <QuickStart />
        <AIDisclosure />
      </main>
    </Layout>
  );
}
