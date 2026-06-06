import { Component, type ReactNode } from 'react';
import { DegradedPage } from '../pages/DegradedPage';

interface Props {
  readonly children: ReactNode;
}
interface State {
  readonly failed: boolean;
}

// Structural backstop for the "never blank" guarantee (spec §10): the data layer degrades on
// anticipated failures, but a render-time throw (e.g. an unexpected/missing field shape) would
// otherwise unmount the whole tree and white-screen. This catches it and shows the degraded
// view pointing at the GitHub source.
export class ErrorBoundary extends Component<Props, State> {
  override state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  override render(): ReactNode {
    if (this.state.failed) {
      return (
        <DegradedPage reason="malformed" githubUrl="https://github.com/GetEvinced/stark-marketplace" />
      );
    }
    return this.props.children;
  }
}
