import React from 'react';
import Link from '@docusaurus/Link';

const FEATURES = [
  {
    emoji: '🔗',
    title: 'Short Links',
    description:
      'Create memorable slugs like go/slack or go/deploy that redirect to any URL. Share them with your team.',
  },
  {
    emoji: '🌐',
    title: 'Browser Extension',
    description:
      'Type bare hostnames like go/jira directly in your browser. Works in Chrome, Firefox, and Safari.',
  },
  {
    emoji: '⌨️',
    title: 'Keyword Templates',
    description:
      'Parameterized links: go/jira/{ticket} expands to your Jira URL with the ticket filled in automatically.',
  },
  {
    emoji: '🔑',
    title: 'REST API & PATs',
    description:
      'Full REST API with Personal Access Tokens for scripting, CI/CD, and automation.',
  },
  {
    emoji: '🔒',
    title: 'Visibility Controls',
    description:
      'Public, private, or secure links. Share individual links with specific users.',
  },
  {
    emoji: '👥',
    title: 'Co-ownership',
    description:
      'Multiple users can own and manage a link — perfect for team-shared resources.',
  },
];

interface HomepageLandingProps {
  adrCount: number;
  specCount: number;
  hasGuides: boolean;
}

export default function HomepageLanding({
  adrCount,
  specCount,
  hasGuides,
}: HomepageLandingProps): JSX.Element {
  return (
    <div className="homepage-landing">
      {/* Hero */}
      <div className="homepage-hero">
        <div className="homepage-hero__badge">Self-hosted go-links</div>
        <h1 className="homepage-hero__title">joe&#x2011;links</h1>
        <p className="homepage-hero__tagline">
          Short memorable URLs for your homelab or team.
          <br />
          Single binary. OIDC auth. No drama.
        </p>
        <div className="homepage-hero__ctas">
          {hasGuides && (
            <Link
              className="button button--primary button--lg"
              to="/guides/getting-started"
            >
              Get Started →
            </Link>
          )}
          <Link
            className="button button--outline button--secondary button--lg"
            href="https://github.com/joestump/joe-links"
          >
            GitHub ↗
          </Link>
        </div>
      </div>

      {/* Feature Tiles */}
      <div className="homepage-features">
        {FEATURES.map((f) => (
          <div key={f.title} className="homepage-feature-tile">
            <div className="homepage-feature-tile__icon">{f.emoji}</div>
            <h3 className="homepage-feature-tile__title">{f.title}</h3>
            <p className="homepage-feature-tile__desc">{f.description}</p>
          </div>
        ))}
      </div>

      {/* Architecture Stats Strip */}
      {(adrCount > 0 || specCount > 0 || hasGuides) && (
        <div className="homepage-stats">
          {hasGuides && (
            <Link className="homepage-stats__card" to="/guides">
              <span className="homepage-stats__icon">📖</span>
              <span className="homepage-stats__label">Guides</span>
            </Link>
          )}
          {adrCount > 0 && (
            <Link className="homepage-stats__card" to="/decisions">
              <span className="homepage-stats__icon">{adrCount}</span>
              <span className="homepage-stats__label">Architecture Decisions</span>
            </Link>
          )}
          {specCount > 0 && (
            <Link className="homepage-stats__card" to="/specs">
              <span className="homepage-stats__icon">{specCount}</span>
              <span className="homepage-stats__label">Specifications</span>
            </Link>
          )}
        </div>
      )}
    </div>
  );
}
