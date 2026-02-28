import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import screenshot from '../screenshots/joe-links-my-links.png';
import styles from './index.module.css';

// ─── Feature list ────────────────────────────────────────────────────────────

type FeatureItem = {
  emoji: string;
  title: string;
  description: JSX.Element;
};

const FeatureList: FeatureItem[] = [
  {
    emoji: '📱',
    title: 'Stop Texting That URL',
    description: (
      <>
        <code>go/plex</code>. <code>go/photos</code>. <code>go/nas</code>.
        Your brother-in-law will still ask for the streaming link, but now you
        can say two words and hang up.
      </>
    ),
  },
  {
    emoji: '👋',
    title: 'Onboard Your New "Users"',
    description: (
      <>
        New friend? Partner? Someone who just got Tailscale access? Drop them a
        link to <code>go/start</code> and they'll figure the rest out.
        Probably.
      </>
    ),
  },
  {
    emoji: '🔄',
    title: "URLs Change. Shortcuts Don't.",
    description: (
      <>
        Migrated from Nginx to Caddy? Rebuilt your NAS? Your friends won't
        notice — the go-links still work. Just update the URL behind the slug.
        Zero retraining required.
      </>
    ),
  },
  {
    emoji: '🏷️',
    title: 'Tag Your Entire Self-Hosted Empire',
    description: (
      <>
        Organize links by service, project, or level of personal embarrassment.
        Browse by tag to find that obscure Gitea webhook you set up at 2am and
        never documented.
      </>
    ),
  },
  {
    emoji: '👥',
    title: 'Co-Owners (Both of You)',
    description: (
      <>
        Share ownership of a link with your one trusted co-admin. If you ever
        get hit by a bus, <code>go/homelab</code> will still work for the
        remaining five users of your platform.
      </>
    ),
  },
  {
    emoji: '⌨️',
    title: 'Parameterized Shortcuts',
    description: (
      <>
        <code>go/jellyfin/&#123;show&#125;</code>,{' '}
        <code>go/gh/&#123;repo&#125;</code> — because{' '}
        <em>technically</em> you only have one user asking for these, but
        making it a template felt really cool.
      </>
    ),
  },
];

// ─── Tech stack ──────────────────────────────────────────────────────────────

type TechItem = {
  name: string;
  label: string;
};

const TechList: TechItem[] = [
  { name: 'Go',            label: 'Language'  },
  { name: 'chi',           label: 'Router'    },
  { name: 'HTMX',          label: 'Frontend'  },
  { name: 'DaisyUI',       label: 'CSS'       },
  { name: 'SQLite',        label: 'Database'  },
  { name: 'PostgreSQL',    label: 'Database'  },
  { name: 'MySQL',         label: 'Database'  },
  { name: 'OIDC',          label: 'Auth'      },
  { name: 'Single Binary', label: 'Deploy'    },
];

// ─── Components ──────────────────────────────────────────────────────────────

function Feature({ emoji, title, description }: FeatureItem) {
  return (
    <div className={clsx('col col--4', styles.featureCol)}>
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
        <div className={styles.heroBadge}>Enterprise-grade · For your 15 closest contacts</div>
        <Heading as="h1" className={styles.heroTitle}>
          joe&#x2011;links
        </Heading>
        <p className={styles.heroTagline}>
          The same go-link technology used by Fortune 500 companies —
          now available for your homelab and the small group of people
          who actually have Tailscale access.
          <br />
          <span className={styles.heroSubTagline}>
            Self-hosted. Single binary. Your friends still won't use it.
          </span>
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

function HomepageScreenshot(): JSX.Element {
  return (
    <section className={styles.screenshotSection}>
      <div className="container">
        <div className={styles.screenshotWrapper}>
          <img
            src={screenshot}
            alt="joe-links dashboard — a surprisingly professional UI for something only you will use"
            className={styles.screenshot}
          />
        </div>
      </div>
    </section>
  );
}

function HomepageFeatures(): JSX.Element {
  return (
    <section className={styles.features}>
      <div className="container">
        <Heading as="h2" className={styles.featuresHeading}>
          Streamline your organization's link infrastructure
        </Heading>
        <p className={styles.featuresSubheading}>
          Studies show that knowledge workers spend 19% of their day searching
          for information. Your friends spend 100% of their day asking you
          what the Jellyfin URL is. We can fix one of those.
        </p>
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function TechStack(): JSX.Element {
  return (
    <section className={styles.techStack}>
      <div className="container">
        <p className={styles.techStackLabel}>Powered by a stack your friends will never appreciate</p>
        <div className={styles.techList}>
          {TechList.map(({ name, label }) => (
            <div key={name} className={styles.techChip}>
              <span className={styles.techName}>{name}</span>
              <span className={styles.techLabel}>{label}</span>
            </div>
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
      title="Self-hosted go-links for your homelab"
      description="Enterprise-grade go-links for your homelab and the 15 people who have Tailscale access. Self-hosted, single binary, OIDC auth, browser extensions for Chrome, Firefox, and Safari."
    >
      <HomepageHeader />
      <main>
        <HomepageScreenshot />
        <HomepageFeatures />
        <TechStack />
        <QuickStart />
        <AIDisclosure />
      </main>
    </Layout>
  );
}
