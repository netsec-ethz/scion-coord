scionApp
    .factory('loginService', ["$http", "$q", function ($http, $q) {
        return {
            // Log the user in
            login: function (user) {
                return $http.post('/api/login', user).then(function (response) {
                    console.log(response);
                    return response.data;
                });
            },
            logout: function () {
                return $http.post('/api/logout').then(function (response) {
                    console.log(response);
                    return response.data;
                });
            },
            resetPassword: function (email) {
                return $http.post('/api/resetPassword?userEmail=' + email).then(function (response) {
                    console.log(response);
                    return response.data;
                });
            }
        };
    }]);
