import React from 'react';
import { Helmet } from 'react-helmet-async';
import { useSelector } from 'react-redux';
import { Link, Route, Switch, useLocation, useRouteMatch } from 'react-router-dom';
import Sidebar from '../../components/Sidebar';
import Dashboard from './Dashboard';

function isActiveCls(className, isActive, activeClass = 'is-active') {
  return className + (isActive ? ` ${activeClass}` : '');
}

const Modtools = () => {
  const user = useSelector((state) => state.main.user);

  let { path } = useRouteMatch();
  const { pathname } = useLocation();

  if (!(user && user.isAdmin)) {
    return (
      <div className="page-content page-full">
        <h1>Forbidden!</h1>
        <div>
          <Link to="/">Go home</Link>.
        </div>
      </div>
    );
  }

  return (
    <div className="page-content wrap admin">
      <Helmet>
        <title>Admin</title>
      </Helmet>
      <Sidebar />
      <div className="admin-head">
        <h1>Admin</h1>
      </div>
      <div className="admin-dashboard">
        <div className="sidebar">
          <Link className={isActiveCls('sidebar-item', pathname === '/admin')} to={`/admin`}>
            Dashboard
          </Link>
          <Link
            className={isActiveCls('sidebar-item', pathname === '/admin/analytics')}
            to={`/admin/analytics`}
          >
            Analytics
          </Link>
          <Link
            className={isActiveCls('sidebar-item', pathname === '/admin/users')}
            to={`/admin/users`}
          >
            Users
          </Link>
          <Link
            className={isActiveCls('sidebar-item', pathname === '/admin/posts')}
            to={`/admin/posts`}
          >
            Posts
          </Link>
          <Link
            className={isActiveCls('sidebar-item', pathname === '/admin/comments')}
            to={`/admin/comments`}
          >
            Comments
          </Link>
        </div>
        <Switch>
          <Route exact path={path}>
            <Dashboard />
          </Route>
          <Route path="*">
            <div className="admin-content flex flex-center">Not found.</div>
          </Route>
        </Switch>
      </div>
    </div>
  );
};

export default Modtools;
