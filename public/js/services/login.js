scionApp
    .factory('loginService', ["$http", "$q", '$httpParamSerializer', function ($http, $q, $httpParamSerializer) {
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
            },
            resendEmail: function (email){
                return $http({
                    method: 'POST',
                    url: '/api/resendLink',
                    data: $httpParamSerializer({ 'email': email }),
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8' }
                });
            }
        };
    }]);
