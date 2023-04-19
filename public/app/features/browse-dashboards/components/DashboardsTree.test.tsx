import { render as rtlRender, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { Router } from 'react-router-dom';

import { locationService } from '@grafana/runtime';

import { wellFormedDashboard, wellFormedEmptyFolder, wellFormedFolder } from '../fixtures/dashboardsTreeItem.fixture';

import { DashboardsTree } from './DashboardsTree';

function render(...args: Parameters<typeof rtlRender>) {
  const [ui, options] = args;

  rtlRender(<Router history={locationService.getHistory()}>{ui}</Router>, options);
}

describe('browse-dashboards DashboardsTree', () => {
  const WIDTH = 800;
  const HEIGHT = 600;

  const folder = wellFormedFolder();
  const emptyFolderIndicator = wellFormedEmptyFolder();
  const dashboard = wellFormedDashboard();

  it('renders a dashboard item', () => {
    render(<DashboardsTree items={[dashboard]} width={WIDTH} height={HEIGHT} onFolderClick={() => {}} />);
    expect(screen.queryByText(dashboard.item.title)).toBeInTheDocument();
    expect(screen.queryByText('Dashboard')).toBeInTheDocument();
  });

  it('renders a folder item', () => {
    render(<DashboardsTree items={[folder]} width={WIDTH} height={HEIGHT} onFolderClick={() => {}} />);
    expect(screen.queryByText(folder.item.title)).toBeInTheDocument();
    expect(screen.queryByText('Folder')).toBeInTheDocument();
  });

  it('calls onFolderClick when a folder button is clicked', async () => {
    const handler = jest.fn();
    render(<DashboardsTree items={[folder]} width={WIDTH} height={HEIGHT} onFolderClick={handler} />);
    const folderButton = screen.getByLabelText('Collapse folder');
    await userEvent.click(folderButton);

    expect(handler).toHaveBeenCalledWith(folder.item.uid, false);
  });

  it('renders empty folder indicators', () => {
    render(<DashboardsTree items={[emptyFolderIndicator]} width={WIDTH} height={HEIGHT} onFolderClick={() => {}} />);
    expect(screen.queryByText('Empty folder')).toBeInTheDocument();
    expect(screen.queryByText(emptyFolderIndicator.item.kind)).not.toBeInTheDocument();
  });
});
