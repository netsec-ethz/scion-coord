scionApp
    .factory('passwordService', ["$http", "$q", function ($http, $q) {
    return {
        // set the password for a user
        setPassword: function (user) {
            return $http.post('/api/setPassword', user).then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);
