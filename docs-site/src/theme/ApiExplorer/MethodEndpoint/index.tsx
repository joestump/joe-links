import React from 'react';
import BrowserOnly from '@docusaurus/BrowserOnly';

/**
 * Swizzle: wrap MethodEndpoint in BrowserOnly.
 *
 * openapi-explorer initialises a Redux store that isn't available during
 * Docusaurus static site generation (SSR), which causes:
 *   TypeError: Cannot destructure property 'store' of 'i' as it is null
 *
 * Rendering this component only in the browser avoids the SSR crash while
 * preserving the full interactive experience once JavaScript loads.
 */
export default function MethodEndpoint(props: Record<string, unknown>): JSX.Element {
  return (
    <BrowserOnly fallback={<span />}>
      {() => {
        // eslint-disable-next-line @typescript-eslint/no-var-requires
        const Original = require('@theme-original/ApiExplorer/MethodEndpoint').default;
        return <Original {...props} />;
      }}
    </BrowserOnly>
  );
}
