import React, { useEffect } from 'react';
import PropTypes from 'prop-types';
import { useDispatch, useSelector } from 'react-redux';
import { fetchStats, fetchRecentItems } from '../../slices/adminSlice';

const Dashboard = () => {
  const dispatch = useDispatch();
  const stats = useSelector((state) => state.admin.stats);
  //   const recentUsers = useSelector((state) => state.admin.recentUsers);
  //   const recentPosts = useSelector((state) => state.admin.recentPosts);
  //   const recentComments = useSelector((state) => state.admin.recentComments);

  useEffect(() => {
    dispatch(fetchStats());
    dispatch(fetchRecentItems());
  }, [dispatch]);

  return (
    <div className="admin-content">
      <div className="admin-content-head">
        <div className="admin-title">Dashboard</div>
      </div>
      <div className="admin-dashboard-stats">
        <div className="stat-card">
          <h2>Total Users</h2>
          <p>{stats.users}</p>
        </div>
        <div className="stat-card">
          <h2>Total Posts</h2>
          <p>{stats.posts}</p>
        </div>
        <div className="stat-card">
          <h2>Total Comments</h2>
          <p>{stats.comments}</p>
        </div>
      </div>
    </div>
  );
};

Dashboard.propTypes = {
  stats: PropTypes.object,
  recentUsers: PropTypes.array,
  recentPosts: PropTypes.array,
  recentComments: PropTypes.array,
};

export default Dashboard;
